package schema

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// outsideTable is an ID past every table of buildDefectSchema.
const outsideTable = 1 << 20

const buildDefectSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
    <xs:attribute name="x" type="xs:string"/>
    <xs:anyAttribute namespace="##any"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:simpleType name="Code"><xs:restriction base="xs:token"/></xs:simpleType>
  <xs:simpleType name="ShortCode"><xs:restriction base="Code"><xs:maxLength value="4"/></xs:restriction></xs:simpleType>
  <xs:element name="head" type="Base"/>
  <xs:element name="member" type="Derived" substitutionGroup="head"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="head"/><xs:element name="c" type="xs:string"/></xs:sequence></xs:complexType>
    <xs:key name="k"><xs:selector xpath="c"/><xs:field xpath="."/></xs:key>
  </xs:element>
</xs:schema>`

func TestPublishReportsCorruptBuildStateAsInternalInvariant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		corrupt func(t *testing.T, rt *schemaBuild)
		// defect is set when a check that returns a build defect is the
		// first to see the corruption, so the invariant must carry it.
		defect bool
	}{
		{
			name: "particle kind out of range",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				t.Helper()
				model := &rt.Models[rt.ComplexTypes[buildDefectComplexType(t, rt, "Base")].Content]
				model.Particles[0].Kind = particleKind(200)
			},
			defect: true,
		},
		{
			name: "complex base outside the type table",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				t.Helper()
				rt.ComplexTypes[buildDefectComplexType(t, rt, "Derived")].Base = ComplexRef(ComplexTypeID(outsideTable))
			},
			defect: true,
		},
		{
			name: "complex derivation cycle",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				t.Helper()
				base := buildDefectComplexType(t, rt, "Base")
				rt.ComplexTypes[base].Base = ComplexRef(buildDefectComplexType(t, rt, "Derived"))
				rt.ComplexTypes[base].Derivation = derivationKindExtension
			},
		},
		{
			name: "attribute wildcard derivation out of range",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				t.Helper()
				rt.AttributeUseSets[rt.ComplexTypes[buildDefectComplexType(t, rt, "Derived")].Attrs].WildcardDerive = attributeWildcardDerivation(200)
			},
			defect: true,
		},
		{
			name: "simple type availability shorter than the type table",
			corrupt: func(_ *testing.T, rt *schemaBuild) {
				rt.simpleTypeUnavailable = rt.simpleTypeUnavailable[:1]
			},
		},
		{
			name: "substitution head outside the element table",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				t.Helper()
				rt.Elements[buildDefectGlobalElement(t, rt, "member")].SubstitutionGroupAffiliation = ElementID(outsideTable)
			},
			defect: true,
		},
		{
			name: "identity constraint kind out of range",
			corrupt: func(_ *testing.T, rt *schemaBuild) {
				rt.Identities[0].Kind = IdentityKind(200)
			},
			defect: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
			if err := c.compileReachedContentModelsForTest(); err != nil {
				t.Fatal(err)
			}
			tt.corrupt(t, &c.rt)
			published, err := c.publishSchema()
			if published != nil {
				t.Fatal("publication returned a schema for corrupt build state")
			}
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if _, ok := errors.AsType[*buildDefectError](err); tt.defect && !ok {
				t.Fatalf("error = %v, want the build defect as its cause", err)
			}
		})
	}
}

func TestDFAGuardsAndActionsReportCounterOutOfRangeAsInternalInvariant(t *testing.T) {
	t.Parallel()
	counters, caps := []uint32{0}, []uint32{3}
	outside := uint32(1)
	for name, err := range map[string]error{
		"guard slot": func() error {
			_, err := dfaGuardOK(counters, caps, compiledGuard{Kind: compiledGuardExitMin, Slot: outside, N: 1})
			return err
		}(),
		"guard kind": func() error {
			_, err := dfaGuardOK(counters, caps, compiledGuard{Kind: compiledGuardKind(200), Slot: 0, N: 1})
			return err
		}(),
		"cap slot":           addCounterCaps(caps, []compiledGuard{{Kind: compiledGuardLoopMax, Slot: outside, N: 2}}),
		"action counter":     applyDFAActions(counters, caps, []compiledAction{{Kind: compiledActionInc, Slot: outside}}),
		"action without cap": applyDFAActions([]uint32{0, 0}, caps, []compiledAction{{Kind: compiledActionInc, Slot: outside}}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestContentModelStepsReportUnknownKindsAsInternalInvariant(t *testing.T) {
	t.Parallel()
	unknown := contentParticle{Kind: particleKind(200), Occurs: occurrence{Min: 1, Max: 1}}
	for name, err := range map[string]error{
		"lowering": func() error {
			work := newWorkBudget(contentModelWorkBudget, 100)
			model := contentModelNode{Kind: modelSequence}
			return appendParticle(&work, nil, &model, unknown)
		}(),
		"DFA edge": func() error {
			_, err := dfaTransitionKeyOf(unknown, 0)
			return err
		}(),
		"emptiability of an optional particle": func() error {
			analysis, err := newContentModelAnalysis(&schemaBuild{}, func(int) error { return nil })
			if err != nil {
				return err
			}
			_, err = analysis.cachedParticleEmptiable(contentParticle{Kind: particleKind(200)})
			return err
		}(),
		"emptiability of an optional model": func() error {
			analysis, err := newContentModelAnalysis(&schemaBuild{}, func(int) error { return nil })
			if err != nil {
				return err
			}
			_, err = analysis.modelEmptiableValue(contentModelNode{Kind: modelKind(200)})
			return err
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expectDiagnostic(t, compileRuleError(xsderrors.CodeSchemaContentModel, "", err), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestContentRestrictionLookupsReturnTheWorkFailureWhenTheBudgetIsSpent(t *testing.T) {
	t.Parallel()
	limit := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model work exhausted", nil)
	v := newContentRestrictionValidator(&schemaBuild{}, func(int) error { return limit }, nil)
	for name, err := range map[string]error{
		"content model": func() error { _, err := v.requireContentModel(0); return err }(),
		"element name":  func() error { _, err := v.requireElementName(0); return err }(),
		"wildcard":      func() error { _, err := v.requireWildcard(0); return err }(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(err, limit) || isBuildDefect(err) {
				t.Fatalf("error = %v, want the work failure", err)
			}
		})
	}
}

func TestUntypedFailureBecomesInternalInvariantAndDiagnosticsPassThrough(t *testing.T) {
	t.Parallel()
	for _, untyped := range []error{
		fmt.Errorf("walk: %w", newBuildDefect("walk references invalid type")),
		errors.New("plain failure"),
	} {
		got := untypedFailure(untyped)
		expectDiagnostic(t, got, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		if !errors.Is(got, untyped) {
			t.Fatalf("untypedFailure(%v) = %v, want it as the cause", untyped, got)
		}
	}
	limit := fmt.Errorf("located: %w", xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "budget", nil))
	if got := untypedFailure(limit); !errors.Is(got, limit) {
		t.Fatalf("diagnostic = %v, want it unchanged", got)
	}
	expectDiagnostic(t, untypedFailure(limit), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func TestPublishReportsAPlainWorkFailureAsInternalInvariant(t *testing.T) {
	t.Parallel()
	c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
	if err := c.compileReachedContentModelsForTest(); err != nil {
		t.Fatal(err)
	}
	plain := errors.New("work account failed")
	published, err := publishSchema(&c.rt, func(int) error { return plain })
	if published != nil {
		t.Fatal("publication returned a schema after its work account failed")
	}
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if !errors.Is(err, plain) {
		t.Fatalf("error = %v, want the work failure as its cause", err)
	}
}

func TestAuditErrorKeepsLimitsAndInvariantsAndMakesEverythingElseAnInvariant(t *testing.T) {
	t.Parallel()
	limit := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "budget", nil)
	invariant := xsderrors.InternalInvariant("broken", nil)
	expectDiagnostic(t, auditError("", limit), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	if got := auditError("", invariant); !errors.Is(got, invariant) || errors.Unwrap(got) != nil {
		t.Fatalf("invariant = %v, want it unchanged", got)
	}
	for _, err := range []error{
		xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations inconsistent", nil),
		errors.New("restriction is not a subset"),
		newBuildDefect("invalid ID"),
	} {
		got := auditError("subject", err)
		expectDiagnostic(t, got, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		if !errors.Is(got, err) {
			t.Fatalf("auditError(%v) = %v, want it as the cause", err, got)
		}
	}
}

func buildDefectComplexType(t *testing.T, rt *schemaBuild, local string) ComplexTypeID {
	t.Helper()
	name, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("missing type name %q", local)
	}
	id, ok := rt.GlobalTypes[name].Complex()
	if !ok {
		t.Fatalf("type %q is not complex", local)
	}
	return id
}

func builtinSimpleTypeIDOf(t *testing.T, rt *schemaBuild, local string) SimpleTypeID {
	t.Helper()
	name, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, local)
	if !ok {
		t.Fatalf("missing built-in type name %q", local)
	}
	id, ok := rt.GlobalTypes[name].Simple()
	if !ok {
		t.Fatalf("built-in type %q is not simple", local)
	}
	return id
}

func buildDefectGlobalElement(t *testing.T, rt *schemaBuild, local string) ElementID {
	t.Helper()
	name, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("missing element name %q", local)
	}
	return rt.GlobalElements[name]
}

func TestDerivationWalksReportMissingTypesAndCyclesAsBuildDefects(t *testing.T) {
	t.Parallel()
	complexWalks := map[string]func(rt *schemaBuild, derived ComplexTypeID) error{
		"substitution blocks": func(rt *schemaBuild, derived ComplexTypeID) error {
			_, err := substitutionTypeBlocks(rt, ComplexRef(derived), ComplexRef(rt.AnyType), unboundedTypeDerivationWork)
			return err
		},
		"derivation to a complex base": func(rt *schemaBuild, derived ComplexTypeID) error {
			_, _, err := complexTypeDerivationMask(rt, derived, rt.AnyType, unboundedTypeDerivationWork)
			return err
		},
		"derivation to anyType": func(rt *schemaBuild, derived ComplexTypeID) error {
			_, _, err := deriveTypeMask(rt, ComplexRef(derived), ComplexRef(rt.AnyType), unboundedTypeDerivationWork)
			return err
		},
		"derivation to a simple base": func(rt *schemaBuild, derived ComplexTypeID) error {
			_, _, err := deriveTypeMask(rt, ComplexRef(derived), SimpleRef(value.AnySimpleTypeID), unboundedTypeDerivationWork)
			return err
		},
	}
	complexCorruptions := map[string]func(rt *schemaBuild, derived, base ComplexTypeID){
		"base outside the type table": func(rt *schemaBuild, derived, _ ComplexTypeID) {
			rt.ComplexTypes[derived].Base = ComplexRef(ComplexTypeID(outsideTable))
		},
		"cycle": func(rt *schemaBuild, derived, base ComplexTypeID) {
			rt.ComplexTypes[base].Base = ComplexRef(derived)
		},
	}
	for walkName, walk := range complexWalks {
		t.Run(walkName+"/intact", func(t *testing.T) {
			t.Parallel()
			c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
			if err := walk(&c.rt, buildDefectComplexType(t, &c.rt, "Derived")); err != nil {
				t.Fatalf("walk error = %v, want none", err)
			}
		})
		for corruptionName, corrupt := range complexCorruptions {
			t.Run(walkName+"/"+corruptionName, func(t *testing.T) {
				t.Parallel()
				c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
				derived, base := buildDefectComplexType(t, &c.rt, "Derived"), buildDefectComplexType(t, &c.rt, "Base")
				corrupt(&c.rt, derived, base)
				if err := walk(&c.rt, derived); !isBuildDefect(err) {
					t.Fatalf("walk error = %v, want a build defect", err)
				}
			})
		}
		t.Run(walkName+"/derived outside the type table", func(t *testing.T) {
			t.Parallel()
			c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
			if err := walk(&c.rt, ComplexTypeID(outsideTable)); !isBuildDefect(err) {
				t.Fatalf("walk error = %v, want a build defect", err)
			}
		})
	}
	baseWalks := map[string]func(rt *schemaBuild, derived, base ComplexTypeID) error{
		"substitution blocks": func(rt *schemaBuild, derived, base ComplexTypeID) error {
			_, err := substitutionTypeBlocks(rt, ComplexRef(derived), ComplexRef(base), unboundedTypeDerivationWork)
			return err
		},
		"derivation to a complex base": func(rt *schemaBuild, derived, base ComplexTypeID) error {
			_, _, err := complexTypeDerivationMask(rt, derived, base, unboundedTypeDerivationWork)
			return err
		},
	}
	for walkName, walk := range baseWalks {
		t.Run(walkName+"/walk base outside the type table", func(t *testing.T) {
			t.Parallel()
			c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
			if err := walk(&c.rt, buildDefectComplexType(t, &c.rt, "Derived"), ComplexTypeID(outsideTable)); !isBuildDefect(err) {
				t.Fatalf("walk error = %v, want a build defect", err)
			}
		})
	}
	t.Run("simple derivation/intact", func(t *testing.T) {
		t.Parallel()
		c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
		derived := buildDefectSimpleType(t, &c.rt, "ShortCode")
		_, found, err := simpleTypeDerivationMaskOf(&c.rt, derived, builtinSimpleTypeIDOf(t, &c.rt, "boolean"), unboundedTypeDerivationWork)
		if err != nil || found {
			t.Fatalf("derivation = %v, %v, want not derived and no error", found, err)
		}
	})
	boolean := value.BuiltinType(value.PrimitiveBoolean)
	baseCycle := cyclicSimpleTypeFixture(qnameRestrictionSpec)
	memberCycle := cyclicSimpleTypeFixture(func(other SimpleTypeID) value.TypeSpec { return unionFixtureSpec(other) })
	simpleCycles := map[string]struct {
		fixture cyclicSimpleTypeFixture
		walk    func(cycle cyclicSimpleTypes) (SimpleTypeID, SimpleTypeID)
	}{
		"base chain cycle to an atomic base": {baseCycle, func(c cyclicSimpleTypes) (SimpleTypeID, SimpleTypeID) { return c.first, boolean }},
		"base chain cycle to a union base":   {baseCycle, func(c cyclicSimpleTypes) (SimpleTypeID, SimpleTypeID) { return c.first, c.union }},
		"union member cycle in the base":     {memberCycle, func(c cyclicSimpleTypes) (SimpleTypeID, SimpleTypeID) { return boolean, c.first }},
	}
	for name, tc := range simpleCycles {
		t.Run("simple derivation/"+name, func(t *testing.T) {
			t.Parallel()
			cycle := tc.fixture.build(t)
			derived, base := tc.walk(cycle)
			if _, _, err := simpleTypeDerivationMaskOf(cycle.rt, derived, base, unboundedTypeDerivationWork); !errors.Is(err, errSimpleDerivationCycle) {
				t.Fatalf("walk error = %v, want %v", err, errSimpleDerivationCycle)
			}
		})
	}
	t.Run("simple derivation/derived outside the type table", func(t *testing.T) {
		t.Parallel()
		c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
		_, _, err := simpleTypeDerivationMaskOf(&c.rt, SimpleTypeID(outsideTable), builtinSimpleTypeIDOf(t, &c.rt, "boolean"), unboundedTypeDerivationWork)
		if !isBuildDefect(err) {
			t.Fatalf("walk error = %v, want a build defect", err)
		}
	})
}

func buildDefectSimpleType(t *testing.T, rt *schemaBuild, local string) SimpleTypeID {
	t.Helper()
	name, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("missing type name %q", local)
	}
	id, ok := rt.GlobalTypes[name].Simple()
	if !ok {
		t.Fatalf("type %q is not simple", local)
	}
	return id
}

func TestCompileRuleChecksReportCorruptInputsAsInternalInvariantAndViolationsAsSchemaErrors(t *testing.T) {
	t.Parallel()
	const invalidMaskBit = DerivationMask(1 << 7)
	c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
	tests := []struct {
		name     string
		err      error
		category xsderrors.Category
		code     xsderrors.Code
	}{
		{"complex final mask outside its domain", checkComplexTypeFinalAllows(invalidMaskBit, DerivationExtension, complexTypeFinalBaseExtension), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant},
		{"complex final blocks extension", checkComplexTypeFinalAllows(DerivationExtension, DerivationExtension, complexTypeFinalBaseExtension), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference},
		{"simple final mask outside its domain", checkSimpleTypeFinalAllows(invalidMaskBit, DerivationRestriction, simpleTypeFinalBaseRestriction), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant},
		{"simple final blocks restriction", checkSimpleTypeFinalAllows(DerivationRestriction, DerivationRestriction, simpleTypeFinalBaseRestriction), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference},
		{"NOTATION use of a type outside the type table", c.admitNotationTypeUse(nil, SimpleTypeID(outsideTable)), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expectDiagnostic(t, tt.err, tt.category, tt.code)
		})
	}
}

// cyclicSimpleTypeFixture builds two simple types that reference each other,
// which value.Builder admits when a facet literal's resolver completes the
// second type while the first is in progress and the first is then completed
// again.
type cyclicSimpleTypeFixture func(other SimpleTypeID) value.TypeSpec

type cyclicSimpleTypes struct {
	rt                   *schemaBuild
	first, second, union SimpleTypeID
}

func (f cyclicSimpleTypeFixture) build(t *testing.T) cyclicSimpleTypes {
	t.Helper()
	b := value.NewBuilder(value.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	var ids [3]SimpleTypeID
	for i := range ids {
		id, err := b.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	first, second, union := ids[0], ids[1], ids[2]
	var nestedErr error
	parentErr := b.Complete(first, value.TypeSpec{
		Base:      value.BuiltinType(value.PrimitiveQName),
		ListItem:  value.NoType,
		Primitive: value.PrimitiveQName,
		Facets: value.FacetSpec{
			Present: value.FacetEnumeration,
			Enumeration: []value.LiteralSpec{{
				Type:    value.NoType,
				Lexical: "p:item",
				Resolver: value.Resolver{QName: func(string) (value.ExpandedName, bool) {
					nestedErr = b.Complete(second, f(first))
					return value.ExpandedName{}, false
				}},
			}},
		},
	})
	if parentErr == nil || nestedErr != nil {
		t.Fatalf("parent failure = %v, nested completion = %v", parentErr, nestedErr)
	}
	if err := b.Complete(first, f(second)); err != nil {
		t.Fatalf("retry first = %v", err)
	}
	if err := b.Complete(union, unionFixtureSpec(value.BuiltinType(value.PrimitiveBoolean))); err != nil {
		t.Fatalf("complete union = %v", err)
	}
	rt := &schemaBuild{SimpleTypes: make([]simpleType, union+1), valueBuilder: b}
	return cyclicSimpleTypes{rt: rt, first: first, second: second, union: union}
}

func qnameRestrictionSpec(base SimpleTypeID) value.TypeSpec {
	return value.TypeSpec{Base: base, ListItem: value.NoType, Primitive: value.PrimitiveQName}
}

func unionFixtureSpec(members ...SimpleTypeID) value.TypeSpec {
	return value.TypeSpec{Base: value.NoType, ListItem: value.NoType, Variety: value.Union, Union: members}
}
