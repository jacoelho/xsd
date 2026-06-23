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

func TestValidateBuiltinAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation)
		wantErr string
	}{
		{name: "valid"},
		{
			name: "missing name",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation) {
				t.Helper()
				delete(names.localIndex, vocab.XMLAttrBase)
			},
			wantErr: "builtin attribute name is missing",
		},
		{
			name: "missing global binding",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation) {
				t.Helper()
				delete(shape.GlobalAttributes, builtinTestXMLQName(t, names, vocab.XMLAttrBase))
			},
			wantErr: "builtin attribute binding does not match declaration",
		},
		{
			name: "declaration name mismatch",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation) {
				t.Helper()
				q := builtinTestXMLQName(t, names, vocab.XMLAttrBase)
				id := shape.GlobalAttributes[q]
				shape.Attributes[id].Name = builtinTestXMLQName(t, names, vocab.XMLAttrID)
			},
			wantErr: "builtin attribute binding does not match declaration",
		},
		{
			name: "handle type mismatch",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation) {
				t.Helper()
				q := builtinTestXMLQName(t, names, vocab.XMLAttrBase)
				shape.Attributes[shape.GlobalAttributes[q]].Type = shape.Builtins.String
			},
			wantErr: "builtin attribute type does not match handle",
		},
		{
			name: "lexical validator mismatch",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation) {
				t.Helper()
				q := builtinTestXMLQName(t, names, vocab.XMLAttrLang)
				typ := shape.Attributes[shape.GlobalAttributes[q]].Type
				shape.SimpleBuiltins[typ] = BuiltinValidationNone
			},
			wantErr: "builtin attribute type does not match lexical validator",
		},
		{
			name: "lexical type out of range",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAttributeValidation) {
				t.Helper()
				q := builtinTestXMLQName(t, names, vocab.XMLAttrLang)
				shape.Attributes[shape.GlobalAttributes[q]].Type = SimpleTypeID(99)
			},
			wantErr: "builtin attribute type does not match lexical validator",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names, shape := builtinAttributeFixture(t)
			if tt.mutate != nil {
				tt.mutate(t, &names, &shape)
			}
			err := ValidateBuiltinAttributes(&names, shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBuiltinAttributes() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBuiltinAttributes() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuiltinAttributeSimpleSeeds(t *testing.T) {
	t.Parallel()

	seeds := BuiltinAttributeSimpleSeeds()
	want := []struct {
		local   string
		builtin BuiltinValidationKind
	}{
		{local: vocab.XMLAttrLang, builtin: BuiltinValidationXMLLang},
		{local: vocab.XMLAttrSpace, builtin: BuiltinValidationXMLSpace},
	}
	if len(seeds) != len(want) {
		t.Fatalf("BuiltinAttributeSimpleSeeds() length = %d, want %d", len(seeds), len(want))
	}
	var internal BuiltinAttributeInternalTypes
	const stringType SimpleTypeID = 7
	for i, seed := range seeds {
		if seed.Namespace != XMLNamespaceURI || seed.Local != want[i].local {
			t.Fatalf("seed %d name = {%q}%q, want {%q}%q", i, seed.Namespace, seed.Local, XMLNamespaceURI, want[i].local)
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
			st.Builtin != want[i].builtin ||
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

func TestValidateBuiltinSimpleTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation)
		wantErr string
	}{
		{name: "valid"},
		{
			name: "invalid explicit builtin ID",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				shape.Builtins.String = SimpleTypeID(99)
			},
			wantErr: "builtin simple type references invalid declaration",
		},
		{
			name: "explicit builtin ID mismatch",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				shape.Builtins.String = shape.Builtins.Boolean
			},
			wantErr: "builtin simple type handle does not match global type",
		},
		{
			name: "missing name",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				delete(names.localIndex, vocab.XSDValueString)
			},
			wantErr: "builtin simple type name is missing",
		},
		{
			name: "missing global binding",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				delete(shape.GlobalTypes, builtinTestXSDQName(t, names, vocab.XSDValueString))
			},
			wantErr: "builtin simple type handle does not match global type",
		},
		{
			name: "wrong global binding kind",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				shape.GlobalTypes[builtinTestXSDQName(t, names, vocab.XSDValueString)] = ComplexRef(0)
			},
			wantErr: "builtin simple type handle does not match global type",
		},
		{
			name: "name drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueString)
				shape.SimpleTypes[id].Name = builtinTestXSDQName(t, names, vocab.XSDValueBoolean)
			},
			wantErr: "builtin simple type name does not match handle: string",
		},
		{
			name: "base drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueString)
				shape.SimpleTypes[id].Base = NoSimpleType
			},
			wantErr: "builtin simple type base does not match handle: string",
		},
		{
			name: "list item drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueIDREFS)
				shape.SimpleTypes[id].ListItem = shape.Builtins.String
			},
			wantErr: "builtin simple type list item does not match handle: IDREFS",
		},
		{
			name: "variety drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueString)
				shape.SimpleTypes[id].Variety = SimpleVarietyList
			},
			wantErr: "builtin simple type variety does not match handle: string",
		},
		{
			name: "primitive drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueString)
				shape.SimpleTypes[id].Primitive = PrimitiveBoolean
			},
			wantErr: "builtin simple type primitive does not match handle: string",
		},
		{
			name: "whitespace drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueString)
				shape.SimpleTypes[id].Whitespace = WhitespaceCollapse
			},
			wantErr: "builtin simple type whitespace does not match handle: string",
		},
		{
			name: "builtin validator drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueLanguage)
				shape.SimpleTypes[id].Builtin = BuiltinValidationNone
			},
			wantErr: "builtin simple type lexical validator does not match handle: language",
		},
		{
			name: "identity drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation) {
				t.Helper()
				id := builtinTestSimpleID(t, names, shape, vocab.XSDValueID)
				shape.SimpleTypes[id].Identity = SimpleIdentityNone
			},
			wantErr: "builtin simple type identity does not match handle: ID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names, shape := builtinSimpleFixture(t)
			if tt.mutate != nil {
				tt.mutate(t, &names, &shape)
			}
			facets, err := ValidateBuiltinSimpleTypes(&names, shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBuiltinSimpleTypes() error = %v", err)
				}
				if len(facets) != len(shape.SimpleTypes) {
					t.Fatalf("ValidateBuiltinSimpleTypes() returned %d facet expectations, want %d", len(facets), len(shape.SimpleTypes))
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBuiltinSimpleTypes() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuiltinValidationForSimpleTypeLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local string
		want  BuiltinValidationKind
	}{
		{local: vocab.XSDValueInteger, want: BuiltinValidationInteger},
		{local: vocab.XSDValueNonNegative, want: BuiltinValidationInteger},
		{local: vocab.XSDValueLanguage, want: BuiltinValidationLanguage},
		{local: vocab.XSDValueName, want: BuiltinValidationName},
		{local: vocab.XSDValueNCName, want: BuiltinValidationNCName},
		{local: vocab.XSDValueID, want: BuiltinValidationNCName},
		{local: vocab.XSDValueIDREF, want: BuiltinValidationNCName},
		{local: vocab.XSDValueNMTOKEN, want: BuiltinValidationNMTOKEN},
		{local: vocab.XSDValueENTITY, want: BuiltinValidationEntity},
		{local: vocab.XSDValueString, want: BuiltinValidationNone},
		{local: "not-a-builtin", want: BuiltinValidationNone},
	}
	for _, tt := range tests {
		got := BuiltinValidationForSimpleTypeLocal(tt.local)
		if got != tt.want {
			t.Fatalf("BuiltinValidationForSimpleTypeLocal(%q) = %v, want %v", tt.local, got, tt.want)
		}
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

func TestValidateBuiltinAnyTypeRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation)
		wantErr string
	}{
		{name: "valid"},
		{
			name: "invalid builtin ID",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.Builtins.AnyType = ComplexTypeID(99)
			},
			wantErr: "builtin anyType references invalid declaration",
		},
		{
			name: "missing name",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				delete(names.localIndex, vocab.XSDValueAnyType)
			},
			wantErr: "builtin anyType name is missing",
		},
		{
			name: "missing global binding",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				delete(shape.GlobalTypes, builtinTestAnyTypeQName(t, names))
			},
			wantErr: "builtin anyType handle does not match global type",
		},
		{
			name: "wrong global binding kind",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.GlobalTypes[builtinTestAnyTypeQName(t, names)] = SimpleRef(0)
			},
			wantErr: "builtin anyType handle does not match global type",
		},
		{
			name: "name drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.ComplexTypes[shape.Builtins.AnyType].Name = QName{}
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "base drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.ComplexTypes[shape.Builtins.AnyType].Base = ComplexRef(shape.Builtins.AnyType)
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "content kind drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.ComplexTypes[shape.Builtins.AnyType].ContentKind = ContentElementOnly
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "text type drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.ComplexTypes[shape.Builtins.AnyType].TextType = 0
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "invalid content model",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.ComplexTypes[shape.Builtins.AnyType].Content = ContentModelID(99)
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "wrong content model kind",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.Models[ct.Content].Kind = ModelEmpty
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "invalid attribute set",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				shape.ComplexTypes[shape.Builtins.AnyType].Attrs = AttributeUseSetID(99)
			},
			wantErr: "builtin anyType shape does not match handle",
		},
		{
			name: "attribute uses",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.AttributeSets[ct.Attrs].UseCount = 1
			},
			wantErr: "builtin anyType attribute set does not match handle",
		},
		{
			name: "attribute index",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.AttributeSets[ct.Attrs].IndexCount = 1
			},
			wantErr: "builtin anyType attribute set does not match handle",
		},
		{
			name: "no wildcard",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.AttributeSets[ct.Attrs].Wildcard = NoWildcard
			},
			wantErr: "builtin anyType attribute set does not match handle",
		},
		{
			name: "invalid wildcard",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.AttributeSets[ct.Attrs].Wildcard = WildcardID(99)
			},
			wantErr: "builtin anyType attribute set does not match handle",
		},
		{
			name: "wildcard mode drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.Wildcards[shape.AttributeSets[ct.Attrs].Wildcard].Mode = WildcardLocal
			},
			wantErr: "builtin anyType attribute wildcard does not match handle",
		},
		{
			name: "wildcard process drift",
			mutate: func(t *testing.T, names *NameTable, shape *BuiltinAnyTypeValidation) {
				t.Helper()
				ct := shape.ComplexTypes[shape.Builtins.AnyType]
				shape.Wildcards[shape.AttributeSets[ct.Attrs].Wildcard].Process = ProcessStrict
			},
			wantErr: "builtin anyType attribute wildcard does not match handle",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names, shape := builtinAnyTypeFixture(t)
			if tt.mutate != nil {
				tt.mutate(t, &names, &shape)
			}
			err := ValidateBuiltinAnyTypeRuntime(&names, shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBuiltinAnyTypeRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBuiltinAnyTypeRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func builtinSimpleFixture(t *testing.T) (NameTable, BuiltinSimpleValidation) {
	t.Helper()

	builtins := builtinSimpleTestIDs()
	expectations := builtinSimpleExpectations(builtins)
	names, err := NewRuntimeNameTable(0)
	if err != nil {
		t.Fatalf("NewRuntimeNameTable() error = %v", err)
	}
	interner := NewNameInterner(&names)
	global := make(map[QName]TypeID, len(expectations))
	idsByLocal := make(map[string]SimpleTypeID, len(expectations))
	var nextID SimpleTypeID
	for _, exp := range expectations {
		id := nextID
		nextID++
		if exp.checkID && exp.id != id {
			t.Fatalf("fixture ID for %s = %d, want %d", exp.local, id, exp.id)
		}
		q, err := interner.InternQName(XSDNamespaceURI, exp.local)
		if err != nil {
			t.Fatalf("InternQName(%q, %q) error = %v", XSDNamespaceURI, exp.local, err)
		}
		global[q] = SimpleRef(id)
		idsByLocal[exp.local] = id
	}
	simpleTypes := make([]BuiltinSimpleDecl, len(expectations))
	nextID = 0
	for _, exp := range expectations {
		id := nextID
		nextID++
		q := builtinTestXSDQName(t, &names, exp.local)
		simpleTypes[id] = BuiltinSimpleDecl{
			Name:       q,
			Base:       builtinTestOptionalSimpleID(idsByLocal, exp.baseLocal),
			ListItem:   builtinTestOptionalSimpleID(idsByLocal, exp.listItemLocal),
			Variety:    exp.variety,
			Primitive:  exp.primitive,
			Whitespace: exp.whitespace,
			Builtin:    exp.builtin,
			Identity:   exp.identity,
		}
	}
	return names, BuiltinSimpleValidation{
		GlobalTypes: global,
		SimpleTypes: simpleTypes,
		Builtins:    builtins,
	}
}

func builtinSimpleTestIDs() BuiltinIDs {
	return BuiltinIDs{
		AnySimpleType: 0,
		String:        1,
		Boolean:       7,
		Decimal:       8,
		Integer:       9,
		Int:           15,
		Date:          25,
		DateTime:      26,
		Time:          27,
		AnyURI:        33,
		QName:         36,
		ID:            38,
		IDREF:         39,
		IDREFS:        40,
		NMTOKEN:       41,
		NMTOKENS:      42,
		ENTITY:        43,
		ENTITIES:      44,
	}
}

func builtinTestOptionalSimpleID(idsByLocal map[string]SimpleTypeID, local string) SimpleTypeID {
	if local == "" {
		return NoSimpleType
	}
	return idsByLocal[local]
}

func builtinAttributeFixture(t *testing.T) (NameTable, BuiltinAttributeValidation) {
	t.Helper()

	const (
		anyURI SimpleTypeID = iota + 1
		id
		stringType
		xmlLang
		xmlSpace
	)
	builtins := BuiltinIDs{
		String: stringType,
		AnyURI: anyURI,
		ID:     id,
	}
	simpleBuiltins := make([]BuiltinValidationKind, xmlSpace+1)
	simpleBuiltins[xmlLang] = BuiltinValidationXMLLang
	simpleBuiltins[xmlSpace] = BuiltinValidationXMLSpace

	names, err := NewRuntimeNameTable(0)
	if err != nil {
		t.Fatalf("NewRuntimeNameTable() error = %v", err)
	}
	interner := NewNameInterner(&names)
	global := make(map[QName]AttributeID)
	var attrs []BuiltinAttributeDecl
	var nextAttr AttributeID
	for _, seed := range builtinAttributeSeedTable {
		exp := builtinAttributeExpectationForSeed(seed, builtins)
		q, err := interner.InternQName(exp.ns, exp.local)
		if err != nil {
			t.Fatalf("InternQName(%q, %q) error = %v", exp.ns, exp.local, err)
		}
		typ := exp.typ
		switch exp.builtin {
		case BuiltinValidationXMLLang:
			typ = xmlLang
		case BuiltinValidationXMLSpace:
			typ = xmlSpace
		case BuiltinValidationNone,
			BuiltinValidationInteger,
			BuiltinValidationName,
			BuiltinValidationNCName,
			BuiltinValidationNMTOKEN,
			BuiltinValidationLanguage,
			BuiltinValidationEntity:
		}
		id := nextAttr
		nextAttr++
		attrs = append(attrs, BuiltinAttributeDecl{Name: q, Type: typ})
		global[q] = id
	}
	return names, BuiltinAttributeValidation{
		GlobalAttributes: global,
		Attributes:       attrs,
		SimpleBuiltins:   simpleBuiltins,
		Builtins:         builtins,
	}
}

func builtinAnyTypeFixture(t *testing.T) (NameTable, BuiltinAnyTypeValidation) {
	t.Helper()

	const (
		anyType ComplexTypeID = iota
	)
	const (
		content ContentModelID = iota
	)
	const (
		attrs AttributeUseSetID = iota
	)
	const (
		wildcard WildcardID = iota
	)

	names, err := NewRuntimeNameTable(0)
	if err != nil {
		t.Fatalf("NewRuntimeNameTable() error = %v", err)
	}
	q, err := NewNameInterner(&names).InternQName(XSDNamespaceURI, vocab.XSDValueAnyType)
	if err != nil {
		t.Fatalf("InternQName(%q, %q) error = %v", XSDNamespaceURI, vocab.XSDValueAnyType, err)
	}
	return names, BuiltinAnyTypeValidation{
		GlobalTypes: map[QName]TypeID{
			q: ComplexRef(anyType),
		},
		ComplexTypes: []ComplexType{
			{
				Name:        q,
				Content:     content,
				Attrs:       attrs,
				TextType:    NoSimpleType,
				ContentKind: ContentMixed,
			},
		},
		Models: []ContentModel{
			{
				Kind:  ModelAny,
				Mixed: true,
			},
		},
		AttributeSets: []BuiltinAnyTypeAttributeSet{
			{Wildcard: wildcard},
		},
		Wildcards: []Wildcard{
			{Mode: WildcardAny, Process: ProcessLax},
		},
		Builtins: BuiltinIDs{AnyType: anyType},
	}
}

func builtinTestSimpleID(t *testing.T, names *NameTable, shape *BuiltinSimpleValidation, local string) SimpleTypeID {
	t.Helper()

	q := builtinTestXSDQName(t, names, local)
	typ, ok := shape.GlobalTypes[q]
	if !ok {
		t.Fatalf("%s global type not found", local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("%s global type is not simple", local)
	}
	return id
}

func builtinTestXSDQName(t *testing.T, names *NameTable, local string) QName {
	t.Helper()

	q, ok := names.LookupQName(XSDNamespaceURI, local)
	if !ok {
		t.Fatalf("%s %s QName not found", XSDNamespaceURI, local)
	}
	return q
}

func builtinTestXMLQName(t *testing.T, names *NameTable, local string) QName {
	t.Helper()

	q, ok := names.LookupQName(XMLNamespaceURI, local)
	if !ok {
		t.Fatalf("%s %s QName not found", XMLNamespaceURI, local)
	}
	return q
}

func builtinTestAnyTypeQName(t *testing.T, names *NameTable) QName {
	t.Helper()

	q, ok := names.LookupQName(XSDNamespaceURI, vocab.XSDValueAnyType)
	if !ok {
		t.Fatalf("%s %s QName not found", XSDNamespaceURI, vocab.XSDValueAnyType)
	}
	return q
}
