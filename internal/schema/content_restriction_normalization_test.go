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
