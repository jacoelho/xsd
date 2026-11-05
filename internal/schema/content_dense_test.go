package schema

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

func TestDenseDFAAdmissionPolicy(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		names []QName
		want  bool
	}{
		{"seven", contentContractNames(7), false}, {"eight", contentContractNames(8), false},
		{"fifteen", contentContractNames(15), false}, {"sixteen", contentContractNames(16), true},
		{"seventeen", contentContractNames(17), true}, {"two hundred", contentContractNames(200), true},
		{"density boundary", append(contentContractNames(15), QName{Local: 32}), true},
		{"too sparse", append(contentContractNames(15), QName{Local: 33}), false},
		{"maximum span", append(contentContractNames(127), QName{Local: 256}), true},
		{"excess span", append(contentContractNames(128), QName{Local: 257}), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, test.names)
			reads := testCompiledModelReads(build.CompiledModels)
			row := &reads[0].Rows[0]
			if (row.indexKind == publishedDFADense) != test.want {
				t.Fatalf("dense admission = %v, want %v", row.indexKind == publishedDFADense, test.want)
			}
			if test.want && (row.index.nameToEdge != nil) {
				t.Fatal("dense row retained fallback index storage")
			}
		})
	}
}

func TestDenseDFAUsesFullQNameAndBounds(t *testing.T) {
	t.Parallel()
	names := contentContractNames(16)
	for i := range uint32(16) {
		names[i] = QName{Namespace: 1000, Local: LocalNameID(math.MaxUint32 - 32 + i*2)}
	}
	build := contentContractBuild(t, names)
	reads := testCompiledModelReads(build.CompiledModels)
	if reads[0].Rows[0].indexKind != publishedDFADense {
		t.Fatal("wide high-ID row was not admitted")
	}
	rt := contentContractSchema(build, reads)
	for _, test := range []struct {
		name  string
		input RuntimeName
		want  ElementID
	}{
		{"first", RuntimeName{Known: true, Name: QName{Namespace: 1000, Local: math.MaxUint32 - 32}}, 0},
		{"middle", RuntimeName{Known: true, Name: QName{Namespace: 1000, Local: math.MaxUint32 - 16}}, 8},
		{"last", RuntimeName{Known: true, Name: QName{Namespace: 1000, Local: math.MaxUint32 - 2}}, 15},
		{"other namespace", RuntimeName{Known: true, Name: QName{Namespace: 1001, Local: math.MaxUint32 - 2}}, NoElement},
		{"unknown", RuntimeName{NS: "urn:unknown", Name: QName{Namespace: 1000, Local: math.MaxUint32 - 2}}, NoElement},
		{"below", RuntimeName{Known: true, Name: QName{Namespace: 1000, Local: 0}}, NoElement},
		{"hole", RuntimeName{Known: true, Name: QName{Namespace: 1000, Local: math.MaxUint32 - 3}}, NoElement},
		{"above", RuntimeName{Known: true, Name: QName{Namespace: 1000, Local: math.MaxUint32 - 1}}, NoElement},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			st := ContentState{model: 0, present: true}
			transition, status := rt.NextContent(st, ContentInput{Name: test.input}, &ContentScratch{})
			kind, element := transition.Match()
			if test.want == NoElement {
				if status != ContentTransitionNoMatch || transition.IsPlanned() {
					t.Fatalf("missing QName = %+v/%v, want no match", transition, status)
				}
				return
			}
			if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != test.want || st.state != 0 {
				t.Fatalf("match = %v/%v/%v, want declaration %d without mutation", status, kind, element, test.want)
			}
			if !transition.Commit(&st, &ContentScratch{}) || st.state != 1 {
				t.Fatalf("commit = %+v, want state 1", st)
			}
		})
	}
}

func TestDenseDFAUniqueCountedRowPreservesAdmission(t *testing.T) {
	t.Parallel()
	build := contentContractBuild(t, contentContractNames(16))
	row := &build.CompiledModels[0].Rows[0]
	row.Accept, row.Counted, row.Min, row.Max = true, true, 1, 1
	row.CountParticle = row.Edges[0].Particle
	row.Edges[0].To = 0
	reads := testCompiledModelReads(build.CompiledModels)
	if reads[0].Rows[0].indexKind != publishedDFADense {
		t.Fatal("unambiguous counted row was not admitted")
	}
	rt := contentContractSchema(build, reads)
	st := ContentState{model: 0, present: true}
	input := ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: 1}}}
	transition, status := rt.NextContent(st, input, &ContentScratch{})
	if status != ContentTransitionMatched || !transition.Commit(&st, &ContentScratch{}) || st.count != 1 {
		t.Fatalf("first counted match = %+v/%v", st, status)
	}
	before := st
	transition, status = rt.NextContent(st, input, &ContentScratch{})
	if status != ContentTransitionNoMatch || transition.IsPlanned() || st != before {
		t.Fatalf("exhausted count = %+v/%v, want no match without mutation", st, status)
	}
	if rt.CompleteContent(st, &ContentScratch{}) != ContentCompletionComplete {
		t.Fatal("satisfied counted row did not complete")
	}
}

func TestDenseDFAExcludesWildcardAndAmbiguousRows(t *testing.T) {
	t.Parallel()
	for _, ambiguous := range []bool{false, true} {
		t.Run(map[bool]string{false: "wildcard", true: "loop and exit"}[ambiguous], func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, contentContractNames(16))
			row := &build.CompiledModels[0].Rows[0]
			if ambiguous {
				loop := row.Edges[0]
				loop.To = 0
				row.Counted, row.Min, row.Max = true, 2, 2
				row.CountParticle = loop.Particle
				row.Edges = append([]CompiledModelEdge{loop}, row.Edges...)
				row.index = dfaRowIndex{}
			} else {
				row.Edges = append(row.Edges, CompiledModelEdge{Particle: WildcardParticle(0, Occurrence{Min: 1, Max: 1}), To: 1})
			}
			if err := IndexCompiledModelsRows(build, build.CompiledModels, unlimitedContentModelWork); err != nil {
				t.Fatal(err)
			}
			reads := testCompiledModelReads(build.CompiledModels)
			if reads[0].Rows[0].indexKind == publishedDFADense {
				t.Fatal("excluded row admitted a dense index")
			}
			read := &reads[0].Rows[0]
			if ambiguous && (read.indexKind != publishedDFALinear || read.index.nameToEdge != nil || read.index.data != nil) {
				t.Fatal("ambiguous row did not retain the canonical linear representation")
			}
			if !ambiguous && read.indexKind != publishedDFAMap {
				t.Fatal("wildcard row lost the canonical map representation")
			}
			if !ambiguous {
				rt := contentContractSchema(build, reads)
				rt.program.Wildcards = []WildcardView{newWildcardView(nil, &Wildcard{Mode: WildcardAny, Process: ProcessSkip})}
				transition, status := rt.NextContent(ContentState{model: 0, present: true}, ContentInput{Name: RuntimeName{NS: "urn:unknown"}}, &ContentScratch{})
				kind, _ := transition.Match()
				if status != ContentTransitionMatched || kind != ContentMatchSkip {
					t.Fatalf("wildcard fallback = %v/%v, want skip", status, kind)
				}
			}
		})
	}
}

func TestDenseDFAEffectiveSubstitutionReturnsActualMember(t *testing.T) {
	t.Parallel()
	for _, exclusion := range []string{"none", "abstract", "blocked"} {
		t.Run(exclusion, func(t *testing.T) {
			t.Parallel()
			expanded := make([]ExpandedName, 17)
			for i := range expanded {
				expanded[i] = ExpandedName{Local: "n" + strconv.Itoa(i)}
			}
			names, err := NewNameTable(18, nil, expanded)
			if err != nil {
				t.Fatal(err)
			}
			elements := make([]ElementDecl, 17)
			for i, name := range expanded {
				qname, _ := names.LookupQName("", name.Local)
				elements[i] = ElementDecl{Name: qname, Type: ComplexRef(0), SubstHead: NoElement}
			}
			elements[16].SubstHead = 0
			elements[16].Abstract = exclusion == "abstract"
			if exclusion == "blocked" {
				elements[0].Block = DerivationSubstitution
			}
			derivations := derivationFixture{anyType: 0, complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}}}
			table, tableErr := BuildSubstitutionTable(derivations.build(), &names, elements, substitutionGlobals(elements), 1, unboundedTypeDerivationWork)
			if tableErr != nil {
				t.Fatal(tableErr)
			}
			build := contentContractBuild(t, contentContractNames(16))
			build.Elements, build.Substitutions = elements, table
			if indexErr := IndexCompiledModelsRows(build, build.CompiledModels, unlimitedContentModelWork); indexErr != nil {
				t.Fatal(indexErr)
			}
			reads := testCompiledModelReads(build.CompiledModels)
			if reads[0].Rows[0].indexKind != publishedDFADense {
				t.Fatal("wide contiguous substitution row was not admitted")
			}
			rt := contentContractSchema(build, reads)
			transition, status := rt.NextContent(ContentState{model: 0, present: true}, ContentInput{Name: RuntimeName{Known: true, Name: elements[16].Name}}, &ContentScratch{})
			kind, member := transition.Match()
			if exclusion == "none" {
				if status != ContentTransitionMatched || kind != ContentMatchDeclared || member != 16 {
					t.Fatalf("substitution = %v/%v/%v, want actual member 16", status, kind, member)
				}
			} else if status != ContentTransitionNoMatch || transition.IsPlanned() {
				t.Fatalf("excluded substitution = %v/%v/%v", status, kind, member)
			}
		})
	}
}

func TestPublishedWidthSixteenContentDispatchIsDense(t *testing.T) {
	t.Parallel()
	published, err := Compile(Options{}, []source.Source{source.Bytes("choice-sixteen.xsd", []byte(publishedWidthSixteenSchema))})
	if err != nil {
		t.Fatal(err)
	}
	rootName, _ := published.LookupQName("", "r")
	root := published.program.GlobalElements[rootName]
	decl, ok := published.program.Elements.start(root)
	if !ok {
		t.Fatal("missing compiled root declaration")
	}
	frame, ok := published.ElementFrame(decl.Type, root)
	if !ok {
		t.Fatal("missing compiled root frame")
	}
	st := frame.Content.ContentState()
	var scratch ContentScratch
	for i := range 16 {
		row := &published.program.CompiledModels[st.model].Rows[st.state]
		if row.indexKind != publishedDFADense || len(row.Edges) != 16 || row.index.nameToEdge != nil {
			t.Fatalf("public width16 state %d did not use dense named dispatch", st.state)
		}
		name, known := published.LookupQName("", "n"+strconv.Itoa(i))
		if !known {
			t.Fatalf("missing child QName n%d", i)
		}
		transition, status := published.NextContent(st, ContentInput{Name: RuntimeName{Known: true, Name: name}}, &scratch)
		kind, element := transition.Match()
		actualName, declared := published.program.Elements.name(element)
		if status != ContentTransitionMatched || kind != ContentMatchDeclared || !declared || actualName != name || !transition.Commit(&st, &scratch) {
			t.Fatalf("public child n%d did not select and commit its declaration", i)
		}
	}
	if published.CompleteContent(st, &scratch) != ContentCompletionComplete {
		t.Fatal("public width16 content did not complete")
	}
	t.Log("exact public16 fixture: initial and repeated child states use dense dispatch")
}

const publishedWidthSixteenSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element name="n0" type="xs:string"/><xs:element name="n1" type="xs:string"/><xs:element name="n2" type="xs:string"/><xs:element name="n3" type="xs:string"/><xs:element name="n4" type="xs:string"/><xs:element name="n5" type="xs:string"/><xs:element name="n6" type="xs:string"/><xs:element name="n7" type="xs:string"/><xs:element name="n8" type="xs:string"/><xs:element name="n9" type="xs:string"/><xs:element name="n10" type="xs:string"/><xs:element name="n11" type="xs:string"/><xs:element name="n12" type="xs:string"/><xs:element name="n13" type="xs:string"/><xs:element name="n14" type="xs:string"/><xs:element name="n15" type="xs:string"/></xs:choice></xs:complexType></xs:element></xs:schema>`

func TestDenseDFAProjectionMarkerAndCompleteSmallFallback(t *testing.T) {
	t.Parallel()
	for _, width := range []int{8, 15, 16} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, contentContractNames(width))
			reads, mayHaveDenseRows, err := newCompiledModelReads(build.CompiledModels, allContentModelIDs(len(build.CompiledModels)), unlimitedContentModelWork)
			if err != nil {
				t.Fatal(err)
			}
			if mayHaveDenseRows != (width == 16) {
				t.Fatalf("optional phase marker = %v for width %d", mayHaveDenseRows, width)
			}
			rt := contentContractSchema(build, reads)
			last := build.Elements[width-1].Name
			build.CompiledModels[0].Rows[0].index.nameToEdge[last] = 0
			build.CompiledModels[0].Rows[0].Edges[width-1].Particle.Element = 0
			transition, status := rt.NextContent(ContentState{model: 0, present: true}, ContentInput{Name: RuntimeName{Known: true, Name: last}}, &ContentScratch{})
			kind, element := transition.Match()
			if status != ContentTransitionMatched || kind != ContentMatchDeclared || int(element) != width-1 {
				t.Fatalf("cloned fallback = %v/%v/%v, want declaration %d", status, kind, element, width-1)
			}
		})
	}
}

func TestPublishedMapIndexPreservesWildcardOrder(t *testing.T) {
	t.Parallel()
	for _, width := range []int{8, 15, 17} {
		for _, before := range []bool{true, false} {
			name := strconv.Itoa(width) + map[bool]string{true: "/before", false: "/after"}[before]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				build := contentContractBuild(t, contentContractNames(width-1))
				row := &build.CompiledModels[0].Rows[0]
				wild := CompiledModelEdge{Particle: WildcardParticle(0, Occurrence{Min: 1, Max: 1}), To: 1}
				if before {
					row.Edges = append([]CompiledModelEdge{wild}, row.Edges...)
				} else {
					row.Edges = append(row.Edges, wild)
				}
				if err := IndexCompiledModelsRows(build, build.CompiledModels, unlimitedContentModelWork); err != nil {
					t.Fatal(err)
				}
				rt := contentContractSchema(build, testCompiledModelReads(build.CompiledModels))
				rt.program.Wildcards = []WildcardView{newWildcardView(nil, &Wildcard{Mode: WildcardAny, Process: ProcessSkip})}
				st := ContentState{model: 0, present: true}
				transition, status := rt.NextContent(st, ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: 1}}}, &ContentScratch{})
				kind, element := transition.Match()
				if status != ContentTransitionMatched || (before && kind != ContentMatchSkip) || (!before && (kind != ContentMatchDeclared || element != 0)) {
					t.Fatalf("ordered fallback = %v/%v/%v", status, kind, element)
				}
				transition, status = rt.NextContent(st, ContentInput{Name: RuntimeName{NS: "urn:unknown"}}, &ContentScratch{})
				kind, _ = transition.Match()
				if status != ContentTransitionMatched || kind != ContentMatchSkip {
					t.Fatalf("unknown-name fallback = %v/%v, want wildcard skip", status, kind)
				}
			})
		}
	}
}

func TestDenseDFAVisitAndArrayAdmissionBounds(t *testing.T) {
	t.Parallel()
	// The first row needs model+row visits (2), N=16 complete scan units,
	// and 2N+S+4=52 array/header units: 70 work and 4*(16+2)=72 bytes.
	for _, test := range []struct {
		name      string
		bytes     uint64
		work      uint64
		remaining uint64
		want      bool
	}{
		{"no model visit", 72, 0, 0, false}, {"no row visit", 72, 1, 0, false},
		{"scan below", 72, 17, 15, false}, {"work below", 72, 69, 51, false},
		{"work at", 72, 70, 0, true}, {"final empty row visit", 72, 71, 0, true},
		{"bytes below", 71, 71, 53, false}, {"bytes above", 73, 71, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, contentContractNames(16))
			reads, _, err := newCompiledModelReads(build.CompiledModels, allContentModelIDs(len(build.CompiledModels)), unlimitedContentModelWork)
			if err != nil {
				t.Fatal(err)
			}
			budget := denseIndexBudget{bytes: test.bytes, work: test.work}
			addDenseDFAIndexes(reads, &budget)
			row := &reads[0].Rows[0]
			if (row.indexKind == publishedDFADense) != test.want || budget.work != test.remaining {
				t.Fatalf("admission = %v, remaining work %d; want %v/%d", row.indexKind == publishedDFADense, budget.work, test.want, test.remaining)
			}
			if test.want {
				if budget.bytes != test.bytes-72 || row.index.nameToEdge != nil {
					t.Fatal("dense admission did not own only its admitted array")
				}
			} else if budget.bytes != test.bytes || row.indexKind != publishedDFAMap {
				t.Fatal("failed admission changed complete fallback storage")
			}
		})
	}
}

func TestDenseDFAExhaustionStopsRemainingConversion(t *testing.T) {
	t.Parallel()
	build := contentContractBuild(t, contentContractNames(200))
	other := contentContractBuild(t, contentContractNames(16))
	models := append([]CompiledModel{build.CompiledModels[0]}, other.CompiledModels...)
	reads, _, err := newCompiledModelReads(models, allContentModelIDs(len(models)), unlimitedContentModelWork)
	if err != nil {
		t.Fatal(err)
	}
	budget := denseIndexBudget{bytes: 72, work: 1000}
	addDenseDFAIndexes(reads, &budget)
	if reads[0].Rows[0].indexKind != publishedDFAMap || reads[1].Rows[0].indexKind != publishedDFAMap || budget.work != 798 || budget.bytes != 72 {
		t.Fatalf("exhaustion did not stop after two visits and 200 scan units: %+v", budget)
	}
}

func TestDenseDFAAggregateAdmissionIsStable(t *testing.T) {
	t.Parallel()
	for _, limit := range []string{"bytes", "work"} {
		t.Run(limit, func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, contentContractNames(16))
			models := []CompiledModel{build.CompiledModels[0], build.CompiledModels[0]}
			reads, _, err := newCompiledModelReads(models, allContentModelIDs(len(models)), unlimitedContentModelWork)
			if err != nil {
				t.Fatal(err)
			}
			budget := defaultDenseIndexBudget()
			if limit == "bytes" {
				budget.bytes = 72
			} else {
				budget.work = 70
			}
			addDenseDFAIndexes(reads, &budget)
			if reads[0].Rows[0].indexKind != publishedDFADense || reads[1].Rows[0].indexKind != publishedDFAMap {
				t.Fatal("aggregate budget did not retain the first dense row and remaining map")
			}
		})
	}
}

func TestDenseDFAWideMapScanIsOrderIndependent(t *testing.T) {
	t.Parallel()
	for rotation := range uint32(32) {
		build := contentContractBuild(t, contentContractNames(16))
		models := []CompiledModel{build.CompiledModels[0], build.CompiledModels[0]}
		reads, _, err := newCompiledModelReads(models, allContentModelIDs(len(models)), unlimitedContentModelWork)
		if err != nil {
			t.Fatal(err)
		}
		index := make(map[QName]uint32, 16)
		for i := range uint32(16) {
			pos := (i + rotation) % 16
			index[QName{Namespace: NamespaceID(pos % 2), Local: LocalNameID(pos + 1)}] = pos
		}
		reads[0].Rows[0].index.nameToEdge = index
		budget := denseIndexBudget{bytes: 72, work: 90}
		addDenseDFAIndexes(reads, &budget)
		if reads[0].Rows[0].indexKind != publishedDFAMap || reads[1].Rows[0].indexKind != publishedDFADense || budget.work != 0 || budget.bytes != 0 {
			t.Fatalf("map traversal changed full scan/visit accounting: %+v", budget)
		}
	}
}

func TestDenseDFAConversionOwnsOneRepresentation(t *testing.T) {
	t.Parallel()
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "dense", true: "map fallback"}[mixed], func(t *testing.T) {
			t.Parallel()
			names := contentContractNames(16)
			if mixed {
				names[15].Namespace = 1
			}
			build := contentContractBuild(t, names)
			reads, _, err := newCompiledModelReads(build.CompiledModels, allContentModelIDs(len(build.CompiledModels)), unlimitedContentModelWork)
			if err != nil {
				t.Fatal(err)
			}
			build.CompiledModels[0].Rows[0].index.nameToEdge[names[15]] = 0
			budget := defaultDenseIndexBudget()
			addDenseDFAIndexes(reads, &budget)
			row := &reads[0].Rows[0]
			if mixed && row.indexKind != publishedDFAMap {
				t.Fatal("mixed names lost map fallback")
			}
			if !mixed && (row.indexKind != publishedDFADense || row.index.nameToEdge != nil) {
				t.Fatal("dense conversion retained a competing map")
			}
			rt := contentContractSchema(build, reads)
			build.CompiledModels[0].Rows[0].Edges[15].Particle.Element = 0
			build.Elements[15].Name = QName{Local: 99}
			transition, status := rt.NextContent(ContentState{model: 0, present: true}, ContentInput{Name: RuntimeName{Known: true, Name: names[15]}}, &ContentScratch{})
			kind, element := transition.Match()
			if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != 15 {
				t.Fatalf("source alias changed publication = %v/%v/%v", status, kind, element)
			}
		})
	}
}

func TestDenseDFAOptionalWorkPreservesMandatoryFailure(t *testing.T) {
	t.Parallel()
	build := contentContractBuild(t, contentContractNames(16))
	build.ComplexTypes = []ComplexType{{}}
	failure := errors.New("mandatory projection allowance")
	remaining := 34
	_, err := newSchemaProgram(build, func(int) error {
		if remaining == 0 {
			return failure
		}
		remaining--
		return nil
	})
	if !errors.Is(err, failure) || remaining != 0 {
		t.Fatalf("mandatory failure = %v, remaining %d", err, remaining)
	}
	calls := 0
	program, err := newSchemaProgram(build, func(steps int) error { calls += steps; return nil })
	if err != nil || calls != 36 || program.CompiledModels[0].Rows[0].indexKind != publishedDFADense {
		t.Fatalf("mandatory retry = %v, work %d; want 36 units and dense admission", err, calls)
	}
}
