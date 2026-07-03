package runtime

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
)

func TestValidateBuiltinDeclarationCounts(t *testing.T) {
	t.Parallel()

	valid := BuiltinDeclarationCounts{
		SimpleTypes:      BuiltinSimpleTypeCount(),
		Attributes:       BuiltinAttributeCount(),
		ComplexTypes:     builtinComplexTypeDeclarationCount,
		Wildcards:        1,
		AttributeUseSets: 1,
		Models:           1,
	}
	tests := []struct {
		name    string
		mutate  func(*BuiltinDeclarationCounts)
		wantErr string
	}{
		{name: "valid"},
		{
			name: "extra declarations",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.SimpleTypes++
				counts.Attributes++
				counts.ComplexTypes++
				counts.Wildcards++
				counts.AttributeUseSets++
				counts.Models++
			},
		},
		{
			name: "missing simple type",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.SimpleTypes--
			},
			wantErr: "runtime is missing builtin declarations",
		},
		{
			name: "missing attribute",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.Attributes--
			},
			wantErr: "runtime is missing builtin declarations",
		},
		{
			name: "missing complex type",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.ComplexTypes--
			},
			wantErr: "runtime is missing builtin declarations",
		},
		{
			name: "missing wildcard",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.Wildcards = 0
			},
			wantErr: "runtime is missing builtin declarations",
		},
		{
			name: "missing attribute use set",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.AttributeUseSets = 0
			},
			wantErr: "runtime is missing builtin declarations",
		},
		{
			name: "missing content model",
			mutate: func(counts *BuiltinDeclarationCounts) {
				counts.Models = 0
			},
			wantErr: "runtime is missing builtin declarations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			counts := valid
			if tt.mutate != nil {
				tt.mutate(&counts)
			}
			err := ValidateBuiltinDeclarationCounts(counts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBuiltinDeclarationCounts() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBuiltinDeclarationCounts() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuiltinAttributeSimpleSeedAccessors(t *testing.T) {
	t.Parallel()

	want := []struct {
		local   string
		builtin BuiltinValidationKind
	}{
		{local: vocab.XMLAttrLang, builtin: BuiltinValidationXMLLang},
		{local: vocab.XMLAttrSpace, builtin: BuiltinValidationXMLSpace},
	}
	if BuiltinAttributeSimpleSeedCount() != len(want) {
		t.Fatalf("BuiltinAttributeSimpleSeedCount() = %d, want %d", BuiltinAttributeSimpleSeedCount(), len(want))
	}
	var internal BuiltinAttributeInternalTypes
	const stringType SimpleTypeID = 7
	for i, expected := range want {
		seed, ok := BuiltinAttributeSimpleSeedAt(i)
		if !ok {
			t.Fatalf("BuiltinAttributeSimpleSeedAt(%d) missing", i)
		}
		if seed.Namespace != XMLNamespaceURI || seed.Local != expected.local {
			t.Fatalf("seed %d name = {%q}%q, want {%q}%q", i, seed.Namespace, seed.Local, XMLNamespaceURI, expected.local)
		}
		base, ok := seed.BaseID(BuiltinIDs{String: stringType})
		if !ok || base != stringType {
			t.Fatalf("seed %d BaseID() = %d, %v; want %d, true", i, base, ok, stringType)
		}
		name := QName{Namespace: NamespaceID(i + 1), Local: LocalNameID(i + 2)}
		st := seed.SimpleType(name, base)
		if st.Name != name ||
			st.Variety != SimpleVarietyAtomic ||
			st.Primitive != PrimitiveString ||
			st.Base != stringType ||
			st.ListItem != NoSimpleType ||
			st.Whitespace != WhitespaceCollapse ||
			st.Builtin != expected.builtin ||
			st.Identity != SimpleIdentityNone ||
			st.Fast != SimpleFastNone {
			t.Fatalf("seed %d SimpleType() = %+v", i, st)
		}
		seed.RecordID(&internal, SimpleTypeID(i+20))
	}
	if internal.XMLLang != 20 || internal.XMLSpace != 21 {
		t.Fatalf("RecordID() = %+v, want XMLLang=20 XMLSpace=21", internal)
	}
}

func TestValidateBuiltinSimpleFacets(t *testing.T) {
	t.Parallel()

	fullExpectation := BuiltinSimpleFacetExpectation{
		MinInclusive:      "0",
		MaxInclusive:      "10",
		MinLength:         1,
		HasFractionDigits: true,
		HasMinLength:      true,
	}
	tests := []struct {
		mutate  func(*BuiltinSimpleFacetValidation)
		name    string
		wantErr string
		exp     BuiltinSimpleFacetExpectation
	}{
		{
			name: "valid no facets",
		},
		{
			name: "valid fixed facets",
			exp:  fullExpectation,
		},
		{
			name: "fixed flag drift",
			exp:  fullExpectation,
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.Fixed = FacetMinLength
			},
			wantErr: "builtin simple type facet flags do not match handle",
		},
		{
			name: "fraction digits drift",
			exp:  fullExpectation,
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.FractionDigits.Value = 1
			},
			wantErr: "builtin integer fractionDigits facet does not match handle",
		},
		{
			name: "min length drift",
			exp:  fullExpectation,
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.MinLength.Value = 2
			},
			wantErr: "builtin list minLength facet does not match handle",
		},
		{
			name: "decimal canonical drift",
			exp:  fullExpectation,
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.MinInclusive.Canonical = "00"
			},
			wantErr: "builtin numeric bound facet does not match handle",
		},
		{
			name: "decimal proof drift",
			exp:  fullExpectation,
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.MinInclusive.ValueMatchesExpected = false
			},
			wantErr: "builtin numeric bound facet does not match handle",
		},
		{
			name: "decimal kind drift",
			exp:  fullExpectation,
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.MaxInclusive.ActualKind = PrimitiveString
			},
			wantErr: "builtin numeric bound facet does not match handle",
		},
		{
			name: "unexpected stored facet",
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.HasLength = true
			},
			wantErr: "builtin simple type stores unexpected facets",
		},
		{
			name: "unexpected enumeration",
			mutate: func(shape *BuiltinSimpleFacetValidation) {
				shape.EnumerationSize = 1
			},
			wantErr: "builtin simple type stores unexpected facets",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			shape := builtinSimpleFacetTestShape(tt.exp)
			if tt.mutate != nil {
				tt.mutate(&shape)
			}
			err := ValidateBuiltinSimpleFacets(shape, tt.exp)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBuiltinSimpleFacets() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBuiltinSimpleFacets() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func builtinSimpleFacetTestShape(exp BuiltinSimpleFacetExpectation) BuiltinSimpleFacetValidation {
	shape := BuiltinSimpleFacetValidation{
		Present: builtinSimpleExpectedFacetMask(exp),
	}
	if exp.HasFractionDigits {
		shape.FractionDigits = BuiltinUnsignedFacet{Present: true}
	}
	if exp.HasMinLength {
		shape.MinLength = BuiltinUnsignedFacet{
			Value:   exp.MinLength,
			Present: true,
		}
	}
	if exp.MinInclusive != "" {
		shape.MinInclusive = BuiltinDecimalFacet{
			Canonical:            exp.MinInclusive,
			ActualKind:           PrimitiveDecimal,
			Present:              true,
			ActualValid:          true,
			ValueMatchesExpected: true,
		}
	}
	if exp.MaxInclusive != "" {
		shape.MaxInclusive = BuiltinDecimalFacet{
			Canonical:            exp.MaxInclusive,
			ActualKind:           PrimitiveDecimal,
			Present:              true,
			ActualValid:          true,
			ValueMatchesExpected: true,
		}
	}
	return shape
}
