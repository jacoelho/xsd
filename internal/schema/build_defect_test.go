package schema

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

const buildDefectSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
    <xs:attribute name="x" type="xs:string"/>
    <xs:anyAttribute namespace="##any"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
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
				model := &rt.Models[rt.ComplexTypes[buildDefectComplexType(t, rt, "Base")].Content]
				model.Particles[0].Kind = particleKind(200)
			},
			defect: true,
		},
		{
			name: "complex base outside the type table",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				rt.ComplexTypes[buildDefectComplexType(t, rt, "Derived")].Base = ComplexRef(ComplexTypeID(len(rt.ComplexTypes) + 7))
			},
			defect: true,
		},
		{
			name: "complex derivation cycle",
			corrupt: func(t *testing.T, rt *schemaBuild) {
				base := buildDefectComplexType(t, rt, "Base")
				rt.ComplexTypes[base].Base = ComplexRef(buildDefectComplexType(t, rt, "Derived"))
				rt.ComplexTypes[base].Derivation = derivationKindExtension
			},
		},
		{
			name: "attribute wildcard derivation out of range",
			corrupt: func(t *testing.T, rt *schemaBuild) {
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
				rt.Elements[buildDefectGlobalElement(t, rt, "member")].SubstHead = ElementID(len(rt.Elements) + 7)
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
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestUntranslatedBuildDefectBecomesInternalInvariantAndDiagnosticsPassThrough(t *testing.T) {
	t.Parallel()
	defect := newBuildDefect("walk references invalid type")
	got := untranslatedBuildDefect(fmt.Errorf("walk: %w", defect))
	expectDiagnostic(t, got, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if !errors.Is(got, defect) {
		t.Fatalf("error = %v, want the defect as its cause", got)
	}
	limit := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "budget", nil)
	if got := untranslatedBuildDefect(limit); got != limit {
		t.Fatalf("diagnostic = %v, want it unchanged", got)
	}
	violation := errors.New("schema violation")
	if got := untranslatedBuildDefect(violation); got != violation {
		t.Fatalf("plain error = %v, want it unchanged", got)
	}
}

func TestAuditErrorKeepsLimitsAndInvariantsAndMakesEverythingElseAnInvariant(t *testing.T) {
	t.Parallel()
	limit := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "budget", nil)
	invariant := xsderrors.InternalInvariant("broken", nil)
	if got := auditError("", limit); got != limit {
		t.Fatalf("limit = %v, want it unchanged", got)
	}
	if got := auditError("", invariant); got != invariant {
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
	walks := map[string]func(rt *schemaBuild, derived, base ComplexTypeID) error{
		"substitution blocks": func(rt *schemaBuild, derived, base ComplexTypeID) error {
			_, err := substitutionTypeBlocks(rt, ComplexRef(derived), ComplexRef(base), unboundedTypeDerivationWork)
			return err
		},
		"derivation to a complex base": func(rt *schemaBuild, derived, base ComplexTypeID) error {
			_, _, err := deriveTypeMask(rt, ComplexRef(derived), ComplexRef(base), unboundedTypeDerivationWork)
			return err
		},
		"derivation to anyType": func(rt *schemaBuild, derived, _ ComplexTypeID) error {
			_, _, err := deriveTypeMask(rt, ComplexRef(derived), ComplexRef(rt.AnyType), unboundedTypeDerivationWork)
			return err
		},
		"derivation to a simple base": func(rt *schemaBuild, derived, _ ComplexTypeID) error {
			_, _, err := deriveTypeMask(rt, ComplexRef(derived), SimpleRef(value.AnySimpleTypeID), unboundedTypeDerivationWork)
			return err
		},
	}
	corruptions := map[string]func(rt *schemaBuild, derived, base ComplexTypeID){
		"base outside the type table": func(rt *schemaBuild, derived, _ ComplexTypeID) {
			rt.ComplexTypes[derived].Base = ComplexRef(ComplexTypeID(len(rt.ComplexTypes) + 7))
		},
		"cycle": func(rt *schemaBuild, derived, base ComplexTypeID) {
			rt.ComplexTypes[base].Base = ComplexRef(derived)
		},
	}
	for walkName, walk := range walks {
		for corruptionName, corrupt := range corruptions {
			t.Run(walkName+"/"+corruptionName, func(t *testing.T) {
				t.Parallel()
				c, _ := newEmptyComplexTypePublicationCompiler(t, buildDefectSchema)
				derived, base := buildDefectComplexType(t, &c.rt, "Derived"), buildDefectComplexType(t, &c.rt, "Base")
				corrupt(&c.rt, derived, base)
				if err := walk(&c.rt, derived, ComplexTypeID(len(c.rt.ComplexTypes)+9)); !isBuildDefect(err) {
					t.Fatalf("walk error = %v, want a build defect", err)
				}
			})
		}
	}
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
		{"NOTATION use of a type outside the type table", c.admitNotationTypeUse(nil, SimpleTypeID(len(c.rt.SimpleTypes)+7)), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectDiagnostic(t, tt.err, tt.category, tt.code)
		})
	}
}
