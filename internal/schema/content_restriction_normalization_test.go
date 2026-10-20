package schema

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestContentRestrictionSubstitutionNormalizationPreservesBuild(t *testing.T) {
	t.Parallel()

	budgetExceeded := errors.New("restriction work exhausted")
	for _, abstract := range []bool{false, true} {
		for _, test := range []struct {
			name     string
			mismatch bool
			exhaust  bool
		}{
			{name: "valid"},
			{name: "namespace mismatch", mismatch: true},
			{name: "work exhausted", exhaust: true},
		} {
			for _, warm := range []bool{false, true} {
				t.Run(fmt.Sprintf("abstract=%t/%s/warm=%t", abstract, test.name, warm), func(t *testing.T) {
					t.Parallel()
					rt, base, derived := substitutionRestrictionFixture(t, abstract)
					if test.mismatch {
						name, ok := rt.Names.LookupQName("urn:heads", "H")
						if !ok {
							t.Fatal("missing head name")
						}
						particles := rt.Models[base].Particles
						if len(particles) != 1 || particles[0].Kind != ParticleWildcard {
							t.Fatal("base is not the expected single wildcard")
						}
						rt.Wildcards[particles[0].Wildcard] = Wildcard{Mode: WildcardOther, Process: ProcessSkip, OtherThan: name.Namespace}
					}
					before := snapshotRestrictionBuild(rt)
					semantics := restrictionModelSemantics(t, rt, unlimitedContentModelAnalysis(rt))
					allowWork := true
					work := func(int) error {
						if !allowWork {
							return budgetExceeded
						}
						return nil
					}
					analysis := mustContentModelAnalysis(t, rt, work)
					if warm {
						_ = restrictionModelSemantics(t, rt, analysis)
					}
					allowWork = !test.exhaust
					err := ValidateContentRestriction(rt, base, derived, work, analysis)
					switch {
					case test.exhaust:
						if !errors.Is(err, budgetExceeded) {
							t.Fatalf("ValidateContentRestriction() error = %v, want original budget error", err)
						}
					case test.mismatch:
						if !IsContentRestrictionMismatch(err) {
							t.Fatalf("ValidateContentRestriction() error = %v, want namespace restriction mismatch", err)
						}
					case err != nil:
						t.Fatalf("ValidateContentRestriction() error = %v", err)
					}
					allowWork = true
					assertRestrictionBuildUnchanged(t, rt, before)
					assertRestrictionSemanticsUnchanged(t, rt, analysis, semantics)
				})
			}
		}
	}
}

func TestContentRestrictionSubstitutionWorkFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	for _, abstract := range []bool{false, true} {
		t.Run(strconv.FormatBool(abstract), func(t *testing.T) {
			t.Parallel()
			rt, base, derived := substitutionRestrictionFixture(t, abstract)
			assertRestrictionWorkFailureAllowsRetry(t, rt, base, derived)
		})
	}
}

func TestContentRestrictionChoiceWorkFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	fixture := contentRestrictionFixture{
		models: []ContentModel{
			{Kind: ModelChoice, Occurs: one, Particles: []Particle{
				ElementParticle(0, one), ElementParticle(1, one), ElementParticle(2, one),
				ElementParticle(3, one), ElementParticle(4, one),
			}},
			{Kind: ModelChoice, Occurs: one, Particles: []Particle{
				ElementParticle(1, one), ElementParticle(3, one),
			}},
		},
	}
	for i := range 5 {
		fixture.elements = append(fixture.elements, QName{Namespace: EmptyNamespaceID, Local: LocalNameID(i + 1)})
		fixture.elementRestrictions = append(fixture.elementRestrictions, ParticleRestrictionElement{
			Type: ComplexRef(0), Scope: DeclarationScopeNonGlobal,
		})
	}
	assertRestrictionWorkFailureAllowsRetry(t, fixture.build(), 0, 1)
}

func TestContentRestrictionRecursiveChoiceWorkFailureAllowsRetry(t *testing.T) {
	t.Parallel()
	one := Occurrence{Min: 1, Max: 1}
	fixture := contentRestrictionRuntimeWith([]ContentModel{
		{Kind: ModelChoice, Occurs: Occurrence{Min: 2, Max: 3}, Particles: []Particle{ElementParticle(1, one), ElementParticle(2, one)}},
		{Kind: ModelChoice, Occurs: Occurrence{Min: 2, Max: 2}, Particles: []Particle{ElementParticle(1, one), ElementParticle(2, one)}},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(3, one), ModelParticle(0, one)}},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(3, one), ModelParticle(1, one)}},
		{Kind: ModelChoice, Occurs: one, Particles: []Particle{ModelParticle(2, one), ElementParticle(4, one)}},
		{Kind: ModelChoice, Occurs: one, Particles: []Particle{ModelParticle(3, one), ElementParticle(4, one)}},
	})
	for i := 3; i <= 4; i++ {
		fixture.elements = append(fixture.elements, QName{Namespace: EmptyNamespaceID, Local: LocalNameID(i)})
		fixture.elementRestrictions = append(fixture.elementRestrictions, ParticleRestrictionElement{Type: ComplexRef(0), Scope: DeclarationScopeNonGlobal})
	}
	assertRestrictionWorkFailureAllowsRetry(t, fixture.build(), 4, 5)
}

func TestContentRestrictionEmptyAgainstEmptiableChoice(t *testing.T) {
	t.Parallel()
	one := Occurrence{Min: 1, Max: 1}
	for _, test := range []struct {
		name      string
		epsilon   bool
		malformed bool
	}{
		{name: "empty branch", epsilon: true},
		{name: "required branch only"},
		{name: "malformed base", epsilon: true, malformed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			element := ElementID(1)
			if test.malformed {
				element = 99
			}
			children := []Particle{ElementParticle(element, one)}
			if test.epsilon {
				children = append(children, ModelParticle(0, one))
			}
			fixture := contentRestrictionRuntimeWith([]ContentModel{
				{Kind: ModelEmpty},
				{Kind: ModelChoice, Occurs: one, Particles: children},
				{Kind: ModelEmpty},
			})
			rt := fixture.build()
			err := ValidateContentRestriction(rt, 1, 2, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt))
			switch {
			case test.malformed:
				if !IsContentRestrictionInvariant(err) {
					t.Fatalf("restriction error = %v, want graph invariant", err)
				}
			case !test.epsilon:
				if !IsContentRestrictionMismatch(err) {
					t.Fatalf("restriction error = %v, want nonemptiable base mismatch", err)
				}
			case err != nil:
				t.Fatalf("empty restriction of emptiable base: %v", err)
			default:
				assertRestrictionWorkFailureAllowsRetry(t, rt, 1, 2)
			}
		})
	}
}

func TestContentRestrictionBodyOccurrences(t *testing.T) {
	t.Parallel()
	one := Occurrence{Min: 1, Max: 1}
	type testCase struct {
		name         string
		body         Occurrence
		baseOuter    Occurrence
		derivedOuter Occurrence
		baseOptional bool
		invalid      bool
	}
	for _, kind := range []ModelKind{ModelChoice, ModelSequence} {
		tests := []testCase{
			{name: "tightened body", body: Occurrence{Min: 2, Max: 2}, baseOuter: one, derivedOuter: one},
			{name: "too few", body: one, baseOuter: one, derivedOuter: one, invalid: true},
			{name: "too many", body: Occurrence{Min: 2, Max: 4}, baseOuter: one, derivedOuter: one, invalid: true},
		}
		if kind == ModelSequence {
			tests = append(tests,
				testCase{name: "tightened both layers", body: Occurrence{Min: 2, Max: 2}, baseOuter: Occurrence{Min: 2, Max: 3}, derivedOuter: Occurrence{Min: 2, Max: 2}},
				testCase{name: "outer minimum lost", body: Occurrence{Min: 2, Max: 3}, baseOuter: Occurrence{Min: 2, Max: 3}, derivedOuter: one, invalid: true},
			)
		} else {
			tests = append(tests,
				testCase{name: "mixed occurrence ownership", body: one, baseOuter: one, derivedOuter: Occurrence{Min: 2, Max: 3}},
				testCase{name: "optional body", body: Occurrence{Min: 0, Max: 2}, baseOuter: one, derivedOuter: one, baseOptional: true},
				testCase{name: "introduced empty body", body: Occurrence{Min: 0, Max: 2}, baseOuter: one, derivedOuter: one, invalid: true},
			)
		}
		for _, test := range tests {
			t.Run(fmt.Sprintf("kind=%d/%s", kind, test.name), func(t *testing.T) {
				t.Parallel()
				children := []Particle{ElementParticle(1, one), ElementParticle(2, one)}
				baseBody := Occurrence{Min: 2, Max: 3}
				if test.baseOptional {
					baseBody.Min = 0
				}
				fixture := contentRestrictionRuntimeWith([]ContentModel{
					{Kind: kind, Occurs: baseBody, Particles: children},
					{Kind: kind, Occurs: test.body, Particles: children},
					{Kind: kind, Occurs: one, Particles: []Particle{ModelParticle(0, test.baseOuter), ElementParticle(3, one)}},
					{Kind: kind, Occurs: one, Particles: []Particle{ModelParticle(1, test.derivedOuter), ElementParticle(3, one)}},
				})
				fixture.elements = append(fixture.elements, QName{Namespace: EmptyNamespaceID, Local: 3})
				fixture.elementRestrictions = append(fixture.elementRestrictions, ParticleRestrictionElement{Type: ComplexRef(0), Scope: DeclarationScopeNonGlobal})
				rt := fixture.build()
				err := ValidateContentRestriction(rt, 2, 3, unlimitedContentModelWork, unlimitedContentModelAnalysis(rt))
				if test.invalid {
					if !IsContentRestrictionMismatch(err) {
						t.Fatalf("ValidateContentRestriction() error = %v, want occurrence mismatch", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("ValidateContentRestriction() error = %v", err)
				}
				assertRestrictionWorkFailureAllowsRetry(t, rt, 2, 3)
			})
		}
	}
}

func assertRestrictionWorkFailureAllowsRetry(t *testing.T, rt *schemaBuild, base, derived ContentModelID) {
	t.Helper()

	budgetExceeded := errors.New("restriction work exhausted")
	before := snapshotRestrictionBuild(rt)
	semantics := restrictionModelSemantics(t, rt, unlimitedContentModelAnalysis(rt))
	const maxBudget = 2048
	completed := false
	remaining := 0
	for budget := range maxBudget + 1 {
		remaining = budget
		work := func(steps int) error {
			remaining -= steps
			if remaining < 0 {
				return budgetExceeded
			}
			return nil
		}
		analysis := mustContentModelAnalysis(t, rt, work)
		err := ValidateContentRestriction(rt, base, derived, work, analysis)
		assertRestrictionBuildUnchanged(t, rt, before)
		if err == nil {
			if budget == 0 {
				t.Fatal("nonempty restriction consumed no bounded work")
			}
			remaining = maxBudget
			assertRestrictionSemanticsUnchanged(t, rt, analysis, semantics)
			completed = true
			break
		}
		if !errors.Is(err, budgetExceeded) {
			t.Fatalf("budget %d: ValidateContentRestriction() error = %v, want original budget error", budget, err)
		}
		// Retry the same build and analysis after every charged prefix;
		// failed normalization must not leave partially derived facts.
		remaining = maxBudget
		if err := ValidateContentRestriction(rt, base, derived, work, analysis); err != nil {
			t.Fatalf("budget %d: retry error = %v", budget, err)
		}
		assertRestrictionBuildUnchanged(t, rt, before)
		assertRestrictionSemanticsUnchanged(t, rt, analysis, semantics)
	}
	if !completed {
		t.Fatalf("valid restriction did not finish within %d work units", maxBudget)
	}
}

// All actual members have the head's xs:string type. NSRecurseCheckCardinality
// against ##any/skip 0..unbounded succeeds for every generated enumeration,
// whether the head itself contributes a concrete choice alternative or not.
func substitutionRestrictionFixture(t *testing.T, abstract bool) (rt *schemaBuild, base, derived ContentModelID) {
	t.Helper()
	var text strings.Builder
	text.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:heads" targetNamespace="urn:heads">`)
	fmt.Fprintf(&text, `<xs:element name="H" type="xs:string" abstract="%t"/>`, abstract)
	for i := range 10 {
		fmt.Fprintf(&text, `<xs:element name="m%d" type="xs:string" substitutionGroup="t:H"/>`, i)
	}
	text.WriteString(`<xs:complexType name="Base"><xs:sequence><xs:any minOccurs="0" maxOccurs="unbounded" processContents="skip"/></xs:sequence></xs:complexType>
<xs:complexType name="Derived"><xs:sequence><xs:element ref="t:H"/></xs:sequence></xs:complexType></xs:schema>`)
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.loadOwned([]source.Source{source.Bytes("restriction.xsd", []byte(text.String()))}); err != nil {
		t.Fatal(err)
	}
	if err := c.index(); err != nil {
		t.Fatal(err)
	}
	if err := c.compileGlobals(); err != nil {
		t.Fatal(err)
	}
	model := func(local string) ContentModelID {
		name, ok := c.rt.Names.LookupQName("urn:heads", local)
		if !ok {
			t.Fatalf("type name %q missing", local)
		}
		typ, ok := c.rt.GlobalTypes[name].Complex()
		if !ok {
			t.Fatalf("type %q is not complex", local)
		}
		return c.rt.ComplexTypes[typ].Content
	}
	base, derived = model("Base"), model("Derived")
	return &c.rt, base, derived
}

type restrictionBuildSnapshot struct {
	models         []ContentModel
	elements       []ElementDecl
	complexTypes   []ComplexType
	wildcards      []Wildcard
	substitutions  SubstitutionTable
	globalElements map[QName]ElementID
	globalTypes    map[QName]TypeID
}

func snapshotRestrictionBuild(rt *schemaBuild) restrictionBuildSnapshot {
	before := restrictionBuildSnapshot{
		models:         make([]ContentModel, len(rt.Models)),
		elements:       slices.Clone(rt.Elements),
		complexTypes:   slices.Clone(rt.ComplexTypes),
		wildcards:      make([]Wildcard, len(rt.Wildcards)),
		substitutions:  SubstitutionTable{spans: slices.Clone(rt.Substitutions.spans), entries: slices.Clone(rt.Substitutions.entries)},
		globalElements: maps.Clone(rt.GlobalElements),
		globalTypes:    maps.Clone(rt.GlobalTypes),
	}
	for i, model := range rt.Models {
		before.models[i] = CloneContentModel(model)
	}
	for i, declaration := range rt.Elements {
		before.elements[i].Identity = slices.Clone(declaration.Identity)
		before.elements[i].Default = cloneValueConstraint(declaration.Default)
		before.elements[i].Fixed = cloneValueConstraint(declaration.Fixed)
	}
	for i, wildcard := range rt.Wildcards {
		before.wildcards[i] = CloneWildcard(wildcard)
	}
	return before
}

func assertRestrictionBuildUnchanged(t *testing.T, rt *schemaBuild, before restrictionBuildSnapshot) {
	t.Helper()
	if !reflect.DeepEqual(snapshotRestrictionBuild(rt), before) {
		t.Fatal("restriction checking changed original models, declaration associations, or substitution members")
	}
}

type restrictionModelSemantic struct {
	count     Occurrence
	emptiable bool
}

func restrictionModelSemantics(t *testing.T, rt *schemaBuild, analysis *ContentModelAnalysis) []restrictionModelSemantic {
	t.Helper()
	facts := make([]restrictionModelSemantic, len(rt.Models))
	for i := range rt.Models {
		id := ContentModelID(i)
		count, err := analysis.ModelCountRange(id)
		if err != nil {
			t.Fatalf("model %d count analysis: %v", i, err)
		}
		emptiable, err := analysis.ModelEmptiable(id)
		if err != nil {
			t.Fatalf("model %d emptiability analysis: %v", i, err)
		}
		facts[i] = restrictionModelSemantic{count: count, emptiable: emptiable}
	}
	return facts
}

func assertRestrictionSemanticsUnchanged(t *testing.T, rt *schemaBuild, analysis *ContentModelAnalysis, want []restrictionModelSemantic) {
	t.Helper()
	if got := restrictionModelSemantics(t, rt, analysis); !slices.Equal(got, want) {
		t.Fatalf("reused model analysis changed: got %+v, want %+v", got, want)
	}
	if got := restrictionModelSemantics(t, rt, unlimitedContentModelAnalysis(rt)); !slices.Equal(got, want) {
		t.Fatalf("fresh model analysis changed: got %+v, want %+v", got, want)
	}
}

func TestContentRestrictionWildcardUsesAggregateMinimum(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		members string
		invalid bool
	}{
		{name: "optional member", members: `<xs:element name="a"/><xs:element name="b" minOccurs="0"/>`},
		{name: "aggregate minimum", members: `<xs:element name="a" minOccurs="0"/><xs:element name="b" minOccurs="0"/>`, invalid: true},
		{name: "aggregate maximum", members: `<xs:element name="a"/><xs:element name="b"/><xs:element name="c"/>`, invalid: true},
		{name: "excluded namespace", members: `<xs:element name="a"/><xs:element name="b" minOccurs="0" form="unqualified"/>`, invalid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			text := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:allowed" targetNamespace="urn:allowed" elementFormDefault="qualified">
<xs:complexType name="Base"><xs:sequence><xs:any namespace="urn:allowed" minOccurs="1" maxOccurs="2" processContents="skip"/></xs:sequence></xs:complexType>
<xs:complexType name="Derived"><xs:complexContent><xs:restriction base="t:Base"><xs:all>` + test.members + `</xs:all></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`
			_, err := Compile(Options{}, []source.Source{source.Bytes("restriction.xsd", []byte(text))})
			if !test.invalid {
				if err != nil {
					t.Fatalf("Compile() error = %v", err)
				}
				return
			}
			if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic.Code() != xsderrors.CodeSchemaContentModel {
				t.Fatalf("Compile() error = %v, want content restriction failure", err)
			}
		})
	}
}

func TestContentRestrictionImplicitChoiceOrdering(t *testing.T) {
	t.Parallel()
	ref := func(names ...string) string {
		var text strings.Builder
		for _, name := range names {
			fmt.Fprintf(&text, `<xs:element ref="t:%s"/>`, name)
		}
		return text.String()
	}
	tests := []struct {
		name    string
		base    string
		derived string
		invalid bool
	}{
		{name: "generated base span", base: ref("p", "H", "q"), derived: ref("p", "b", "a", "q")},
		{name: "generated derived span", base: ref("p", "b", "a", "q"), derived: ref("p", "G", "q")},
		{name: "both generated spans", base: ref("p", "H", "q"), derived: ref("p", "G", "q")},
		{name: "explicit sibling reversal", base: ref("p", "H", "q"), derived: ref("q", "b", "a", "p"), invalid: true},
		{name: "cross generated boundary", base: ref("p", "H", "q"), derived: ref("p", "a", "q", "b"), invalid: true},
		{name: "adjacent generated spans", base: ref("p", "H", "J", "q"), derived: ref("p", "b", "a", "e", "d", "q")},
		{name: "cross adjacent generated spans", base: ref("p", "H", "J", "q"), derived: ref("p", "d", "b", "a", "e", "q"), invalid: true},
		{name: "explicit branch reversal", base: ref("p", "a", "b", "q"), derived: ref("p", "b", "a", "q"), invalid: true},
		{name: "nested generated span", base: ref("p") + `<xs:choice>` + ref("H", "q") + `</xs:choice>`, derived: ref("p", "b", "a", "q")},
		{name: "nested boundary crossing", base: ref("p") + `<xs:choice>` + ref("H", "q") + `</xs:choice>`, derived: ref("p", "a", "q", "b"), invalid: true},
		{name: "generated members share wildcard", base: `<xs:any namespace="urn:choice" processContents="skip"/><xs:element name="local" form="unqualified"/>`, derived: ref("G")},
		{name: "explicit members cannot share wildcard", base: `<xs:any namespace="urn:choice" processContents="skip"/><xs:element name="local" form="unqualified"/>`, derived: ref("a", "b"), invalid: true},
	}
	for _, test := range tests {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%t", test.name, reverse), func(t *testing.T) {
				t.Parallel()
				members := []string{"a", "b"}
				if reverse {
					slices.Reverse(members)
				}
				var text strings.Builder
				text.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:choice" targetNamespace="urn:choice">
<xs:element name="H" abstract="true" type="xs:string"/>
<xs:element name="G" abstract="true" type="xs:string" substitutionGroup="t:H"/>
<xs:element name="c" type="xs:string" substitutionGroup="t:H"/>
<xs:element name="p" type="xs:string"/><xs:element name="q" type="xs:string"/>
<xs:element name="J" abstract="true" type="xs:string"/>
<xs:element name="d" type="xs:string" substitutionGroup="t:J"/>
<xs:element name="e" type="xs:string" substitutionGroup="t:J"/>`)
				for _, name := range members {
					fmt.Fprintf(&text, `<xs:element name="%s" type="xs:string" substitutionGroup="t:G"/>`, name)
				}
				fmt.Fprintf(&text, `<xs:complexType name="Base"><xs:choice>%s</xs:choice></xs:complexType>
<xs:complexType name="Derived"><xs:complexContent><xs:restriction base="t:Base"><xs:choice>%s</xs:choice></xs:restriction></xs:complexContent></xs:complexType></xs:schema>`, test.base, test.derived)
				_, err := Compile(Options{}, []source.Source{source.Bytes("restriction.xsd", []byte(text.String()))})
				if !test.invalid {
					if err != nil {
						t.Fatalf("Compile() error = %v", err)
					}
					return
				}
				if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic.Code() != xsderrors.CodeSchemaContentModel {
					t.Fatalf("Compile() error = %v, want content restriction failure", err)
				}
			})
		}
	}
}

func TestContentRestrictionImplicitChoiceWorkFailureAllowsRetry(t *testing.T) {
	t.Parallel()
	rt, base, derived := substitutionRestrictionFixture(t, true)
	one := Occurrence{Min: 1, Max: 1}
	var particles []Particle
	for i := 9; i >= 0; i-- {
		name, ok := rt.Names.LookupQName("urn:heads", fmt.Sprintf("m%d", i))
		if !ok {
			t.Fatal("missing substitution member name")
		}
		id, ok := rt.GlobalElements[name]
		if !ok {
			t.Fatal("missing substitution member")
		}
		particles = append(particles, ElementParticle(id, one))
	}
	rt.Models[base] = ContentModel{Kind: ModelChoice, Occurs: one, Particles: particles}
	assertRestrictionWorkFailureAllowsRetry(t, rt, base, derived)
}
