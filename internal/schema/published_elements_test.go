package schema

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The declarations of a named group compile before every global
// declaration, so unreachedGroup shifts the published ID of every global.
const publishedElementsSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  %s
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" type="xs:string" substitutionGroup="head"/>
  <xs:element name="other" type="xs:string" substitutionGroup="member"/>
  <xs:element name="root"><xs:complexType><xs:sequence>
    %s
    <xs:element name="pair"><xs:complexType><xs:all><xs:element name="c"/><xs:element name="d"/></xs:all></xs:complexType></xs:element>
    <xs:element ref="head" maxOccurs="unbounded"/>
    <xs:element name="counted" minOccurs="2" maxOccurs="20"/>
    <xs:element name="pick"><xs:complexType><xs:choice maxOccurs="unbounded"><xs:element name="p1"/><xs:element name="p2"/></xs:choice></xs:complexType></xs:element>
    <xs:element name="seq"><xs:complexType><xs:sequence>
      <xs:element name="s1" minOccurs="0"/><xs:element name="s2" minOccurs="0"/><xs:element name="s3" minOccurs="0"/>
    </xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const (
	unreachedGroup = `<xs:group name="dead"><xs:sequence>
    <xs:element name="g0" type="xs:int" default="1"/>
    <xs:element name="head" nillable="true"/>
  </xs:sequence></xs:group>`
	unreachedLocals = `<xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>
      <xs:element name="g1"><xs:complexType><xs:all><xs:element name="g2"/></xs:all></xs:complexType></xs:element>
    </xs:sequence></xs:complexType></xs:element>
    <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="g3"/></xs:sequence>`
)

func TestPublicationOmitsUnreachedElementDeclarations(t *testing.T) {
	t.Parallel()

	published := func(t *testing.T, group, locals string) []ExpandedName {
		t.Helper()
		schema, err := publishSchema(compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, group, locals)), unlimitedContentModelWork)
		if err != nil {
			t.Fatalf("publishSchema() error = %v", err)
		}
		names := make([]ExpandedName, len(schema.program.Elements.meta))
		for i := range names {
			name, ok := schema.DiagnosticElementName(ElementID(i))
			if !ok {
				t.Fatalf("DiagnosticElementName(%d) is not published", i)
			}
			names[i] = name
		}
		return names
	}
	want := published(t, "", "")
	if got := published(t, unreachedGroup, unreachedLocals); !slices.Equal(got, want) {
		t.Fatalf("published element declarations = %v, want %v as without unreached declarations", got, want)
	}
}

func TestPublishedElementIDsNameTheDeclarationsTheBuildNamed(t *testing.T) {
	t.Parallel()

	publishedContent := func(t *testing.T, schema *Schema) []ExpandedName {
		t.Helper()
		var names []ExpandedName
		for i := range schema.program.CompiledModels {
			names = appendContentElementNames(names, &schema.program.CompiledModels[i], func(id ElementID) ExpandedName {
				name, ok := schema.DiagnosticElementName(id)
				if !ok {
					t.Fatalf("published content model names element %d, outside the %d published declarations", id, len(schema.program.Elements.meta))
				}
				return name
			})
		}
		return names
	}
	oracle, err := publishSchema(compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, "", "")), unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() without unreached declarations error = %v", err)
	}
	wantContent := publishedContent(t, oracle)

	build := compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals))
	names := newNameReadView(&build.Names)
	buildName := func(id ElementID) ExpandedName {
		q := build.Elements[id].Name
		return ExpandedName{Namespace: names.Namespace(q.Namespace), Local: build.Names.locals[q.Local]}
	}
	if head := build.GlobalElements[mustBuildQName(t, build, "head")]; head == 0 || build.ReachableElements[0] {
		t.Fatalf("fixture: global head has build ID %d after a reached declaration, want unreached declarations first", head)
	}
	counted := false
	for _, root := range enteredContentForTest(t, build).Models {
		for _, row := range build.CompiledContent.Models[root].Rows {
			counted = counted || row.Counted
		}
	}
	if !counted {
		t.Fatal("fixture publishes no counted row")
	}
	wantGlobals := make(map[QName]ExpandedName, len(build.GlobalElements))
	wantMembers := make(map[QName][]ExpandedName, len(build.GlobalElements))
	for q, id := range build.GlobalElements {
		wantGlobals[q] = buildName(id)
		build.Substitutions.ForEachMember(id, func(member ElementID) bool {
			wantMembers[q] = append(wantMembers[q], buildName(member))
			return true
		})
	}

	schema, err := publishSchema(build, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	publishedName := func(id ElementID) ExpandedName {
		name, ok := schema.DiagnosticElementName(id)
		if !ok {
			t.Fatalf("published holder names element %d, outside the %d published declarations", id, len(schema.program.Elements.meta))
		}
		return name
	}
	if gotContent := publishedContent(t, schema); !slices.Equal(gotContent, wantContent) {
		t.Errorf("published content particles name %v, want %v as without unreached declarations", gotContent, wantContent)
	}
	if len(schema.program.GlobalElements) != len(wantGlobals) {
		t.Errorf("published %d global elements, want %d", len(schema.program.GlobalElements), len(wantGlobals))
	}
	for q, want := range wantGlobals {
		if _, ok := schema.program.GlobalElements[q]; !ok {
			t.Errorf("published global elements lack %v", want)
		}
	}
	for q, id := range schema.program.GlobalElements {
		if got := publishedName(id); got != wantGlobals[q] {
			t.Errorf("published global %v names %v", wantGlobals[q], got)
		}
		var members []ExpandedName
		schema.program.Substitutions.ForEachMember(id, func(member ElementID) bool {
			members = append(members, publishedName(member))
			return true
		})
		if !slices.Equal(members, wantMembers[q]) {
			t.Errorf("published substitution members of %v = %v, want %v", wantGlobals[q], members, wantMembers[q])
		}
	}
}

// wideChoice gives the schema a row of more edges than dense conversion
// needs, so publication writes dense indexes into the rows it shares.
const wideChoice = `<xs:element name="wide"><xs:complexType><xs:choice>
      <xs:element name="w01"/><xs:element name="w02"/><xs:element name="w03"/><xs:element name="w04"/><xs:element name="w05"/>
      <xs:element name="w06"/><xs:element name="w07"/><xs:element name="w08"/><xs:element name="w09"/><xs:element name="w10"/>
      <xs:element name="w11"/><xs:element name="w12"/><xs:element name="w13"/><xs:element name="w14"/><xs:element name="w15"/>
      <xs:element name="w16"/><xs:element name="w17"/><xs:element name="w18"/><xs:element name="w19"/><xs:element name="w20"/>
    </xs:choice></xs:complexType></xs:element>`

func TestPublicationRejectedWhenSealingValuesLeavesBuildUnchanged(t *testing.T) {
	t.Parallel()

	for name, schema := range map[string]string{
		"every declaration reached": fmt.Sprintf(publishedElementsSchema, "", wideChoice),
		"unreached declarations":    fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals+wideChoice),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			build, want := compiledTestBuild(t, schema), compiledTestBuild(t, schema)
			for _, b := range []*schemaBuild{build, want} {
				if _, err := b.valueBuilder.Reserve(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := publishSchema(build, unlimitedContentModelWork); err == nil || !strings.Contains(err.Error(), "value program metadata is invalid") {
				t.Fatalf("publishSchema() error = %v, want an incomplete value program rejected when sealed", err)
			}
			if !reflect.DeepEqual(build.CompiledContent, want.CompiledContent) || !reflect.DeepEqual(build.Substitutions, want.Substitutions) {
				t.Fatal("rejected publishSchema() changed the build's content tables or substitution table")
			}
		})
	}
}

func TestPublicationSharesEveryContentTableWithTheBuild(t *testing.T) {
	t.Parallel()

	for name, schema := range map[string]string{
		"every declaration reached":    fmt.Sprintf(publishedElementsSchema, "", ""),
		"unreached declarations first": fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			build := compiledTestBuild(t, schema)
			var buildRows []*compiledModelRowRead
			for _, root := range enteredContentForTest(t, build).Models {
				if rows := build.CompiledContent.Models[root].Rows; len(rows) != 0 {
					buildRows = append(buildRows, &rows[0])
				}
			}
			published, err := publishSchema(build, unlimitedContentModelWork)
			if err != nil {
				t.Fatalf("publishSchema() error = %v", err)
			}
			var rows []*compiledModelRowRead
			for i := range published.program.CompiledModels {
				if modelRows := published.program.CompiledModels[i].Rows; len(modelRows) != 0 {
					rows = append(rows, &modelRows[0])
				}
			}
			if len(rows) != len(buildRows) || len(rows) == 0 {
				t.Fatalf("published %d tables with rows, want %d and at least one", len(rows), len(buildRows))
			}
			for i := range rows {
				if rows[i] != buildRows[i] {
					t.Errorf("published table %d does not share the build's rows", i)
				}
			}
		})
	}
}

func TestPublicationRejectsContentTablesBuiltOverAnotherNumbering(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		corrupt func(*testing.T, *schemaBuild)
	}{
		{name: "unreached declaration marked reached", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			id := slices.Index(build.ReachableElements, false)
			if id < 0 {
				t.Fatal("fixture has no unreached element declaration")
			}
			build.ReachableElements[id] = true
		}},
		{name: "reached local declaration marked unreached", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			for i := range build.Elements {
				if build.ReachableElements[i] && build.Elements[i].Scope != declarationScopeGlobal {
					build.ReachableElements[i] = false
					return
				}
			}
			t.Fatal("fixture has no reached local element declaration")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals))
			test.corrupt(t, build)
			_, err := publishSchema(build, unlimitedContentModelWork)
			if err == nil || !strings.Contains(err.Error(), "compiled content tables number element declarations other than reachability does") {
				t.Fatalf("publishSchema() error = %v, want content tables over another numbering rejected", err)
			}
		})
	}
}

func appendContentElementNames(names []ExpandedName, model *compiledModelRead, name func(ElementID) ExpandedName) []ExpandedName {
	add := func(p compiledParticleRead) {
		if p.Kind == particleElement {
			names = append(names, name(ElementID(p.Element)))
		}
	}
	for _, row := range model.Rows {
		for _, edge := range row.Edges {
			add(edge.Particle)
		}
		if row.Counted {
			add(row.CountParticle)
		}
	}
	for _, term := range model.All {
		add(term.Particle)
	}
	return names
}

func TestPublicationRejectsElementHoldersWithoutPublishedIDs(t *testing.T) {
	t.Parallel()

	unreached := func(t *testing.T, build *schemaBuild) ElementID {
		t.Helper()
		id := slices.Index(build.ReachableElements, false)
		if id < 0 {
			t.Fatal("fixture has no unreached element declaration")
		}
		return ElementID(id) // #nosec G115 -- id indexes an element-declaration table.
	}
	for _, test := range []struct {
		name    string
		corrupt func(*testing.T, *schemaBuild)
		message string
	}{
		{name: "reachability shorter than the declarations", message: "element reachability does not cover the element declarations", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.ReachableElements = build.ReachableElements[:len(build.ReachableElements)-1]
		}},
		{name: "substitution member unreached", message: "substitution member has no published ID", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.Substitutions.entries[0].member = unreached(t, build)
		}},
		{name: "substitution head unreached", message: "substitution head has no published ID", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			head := build.GlobalElements[mustBuildQName(t, build, "head")]
			build.Substitutions.spans[unreached(t, build)] = build.Substitutions.spans[head]
		}},
		{name: "substitution member unreached in a table without spans", message: "substitution member has no published ID", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.Substitutions.entries[0].member = unreached(t, build)
			build.Substitutions.spans = nil
		}},
		{name: "substitution table shorter than the declarations", message: "substitution table does not cover the element declarations", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			build.Substitutions.spans = build.Substitutions.spans[:len(build.Substitutions.spans)-1]
		}},
		{name: "content particle outside the published declarations", message: "published content model references an invalid element declaration", corrupt: func(t *testing.T, build *schemaBuild) {
			t.Helper()
			for _, root := range enteredContentForTest(t, build).Models {
				if all := build.CompiledContent.Models[root].All; len(all) != 0 {
					all[0].Particle.Element = publishedElementID(build.CompiledContent.Elements.count) // #nosec G115 -- count is an element-declaration count.
					return
				}
			}
			t.Fatal("fixture publishes no xs:all model")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals))
			test.corrupt(t, build)
			_, err := publishSchema(build, unlimitedContentModelWork)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("publishSchema() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestPublishedElementMapsRejectAGlobalWithoutPublishedID(t *testing.T) {
	t.Parallel()

	build := &schemaBuild{
		Elements:          make([]elementDecl, 2),
		ReachableElements: []bool{false, true},
		GlobalElements:    map[QName]ElementID{{Local: 1}: 0},
	}
	elements, err := newPublishedElements(build.ReachableElements, len(build.Elements))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := publishedElementMaps(build, elements); err == nil || !strings.Contains(err.Error(), "global element declaration has no published ID") {
		t.Fatalf("publishedElementMaps() error = %v, want a global without a published ID rejected", err)
	}
}

func TestPublicationRejectsEnteredRootWithoutTables(t *testing.T) {
	t.Parallel()

	for name, kind := range map[string]compiledModelKind{"xs:all": compiledModelAll, "automaton": compiledModelDFA} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			build := compiledTestBuild(t, fmt.Sprintf(publishedElementsSchema, unreachedGroup, unreachedLocals))
			stripped := false
			for _, root := range enteredContentForTest(t, build).Models {
				if model := &build.CompiledContent.Models[root]; model.Kind == kind {
					*model = model.withoutTables()
					build.CompiledContent.Runs[root] = nil
					stripped = true
					break
				}
			}
			if !stripped {
				t.Fatalf("fixture enters no %s root", name)
			}
			_, err := publishSchema(build, unlimitedContentModelWork)
			if err == nil || !strings.Contains(err.Error(), "published content model has no tables") {
				t.Fatalf("publishSchema() error = %v, want an entered root without tables rejected", err)
			}
		})
	}
}

// renumberCompiledContent records the numbering of build.ReachableElements
// as the one the compiled content tables were built over.
func renumberCompiledContent(t *testing.T, build *schemaBuild) {
	t.Helper()
	elements, err := newPublishedElements(build.ReachableElements, len(build.Elements))
	if err != nil {
		t.Fatal(err)
	}
	build.CompiledContent.Elements = nil
	if !elements.identity() {
		build.CompiledContent.Elements = &elements
	}
}

func mustBuildQName(t *testing.T, build *schemaBuild, local string) QName {
	t.Helper()
	q, ok := build.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("name %q not interned", local)
	}
	return q
}
