package runtime

import (
	"strings"
	"testing"
)

func TestPrimitiveKindValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []PrimitiveKind{
		PrimitiveString,
		PrimitiveBoolean,
		PrimitiveDecimal,
		PrimitiveFloat,
		PrimitiveDouble,
		PrimitiveDuration,
		PrimitiveDateTime,
		PrimitiveTime,
		PrimitiveDate,
		PrimitiveGYearMonth,
		PrimitiveGYear,
		PrimitiveGMonthDay,
		PrimitiveGDay,
		PrimitiveGMonth,
		PrimitiveHexBinary,
		PrimitiveBase64Binary,
		PrimitiveAnyURI,
		PrimitiveQName,
		PrimitiveNotation,
	} {
		if !ValidPrimitiveKind(kind) {
			t.Fatalf("ValidPrimitiveKind(%d) = false", kind)
		}
	}
	if ValidPrimitiveKind(PrimitiveKind(99)) {
		t.Fatal("invalid primitive kind was accepted")
	}
}

func TestWhitespaceModeValidity(t *testing.T) {
	t.Parallel()

	for _, mode := range []WhitespaceMode{
		WhitespacePreserve,
		WhitespaceReplace,
		WhitespaceCollapse,
	} {
		if !ValidWhitespaceMode(mode) {
			t.Fatalf("ValidWhitespaceMode(%d) = false", mode)
		}
	}
	if ValidWhitespaceMode(WhitespaceMode(99)) {
		t.Fatal("invalid whitespace mode was accepted")
	}
}

func TestMissingSimpleType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 2}
	const base SimpleTypeID = 3
	got := MissingSimpleType(name, base)
	if got.Name != name ||
		got.Variety != SimpleVarietyAtomic ||
		got.Primitive != PrimitiveString ||
		got.Base != base ||
		got.ListItem != NoSimpleType ||
		got.Whitespace != WhitespaceCollapse ||
		got.Identity != SimpleIdentityNone ||
		got.Fast != SimpleFastNone ||
		!got.Missing {
		t.Fatalf("MissingSimpleType() = %+v", got)
	}
	if MissingSimpleTypeLocalName() != "missing" {
		t.Fatalf("MissingSimpleTypeLocalName() = %q", MissingSimpleTypeLocalName())
	}
}

func TestSimpleTypeByID(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		{Name: QName{Local: 1}},
		{Name: QName{Local: 2}},
	}
	got, ok := SimpleTypeByID(types, 1)
	if !ok || got.Name != types[1].Name {
		t.Fatalf("SimpleTypeByID(valid) = %+v, %v; want type 1, true", got, ok)
	}
	for _, id := range []SimpleTypeID{NoSimpleType, 2} {
		got, ok := SimpleTypeByID(types, id)
		if ok || got != nil {
			t.Fatalf("SimpleTypeByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
	}
}

func TestUsableSimpleTypeRejectsMissingSentinel(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		{Name: QName{Local: 1}},
		MissingSimpleType(QName{Local: 2}, 0),
	}
	if got, ok := UsableSimpleType(types, 0); !ok || got.Name != types[0].Name {
		t.Fatalf("UsableSimpleType(valid) = %+v, %v; want type 0, true", got, ok)
	}
	if got, ok := UsableSimpleType(types, 1); ok || got != nil {
		t.Fatalf("UsableSimpleType(missing) = %+v, %v; want nil, false", got, ok)
	}
	if got, ok := UsableSimpleType(types, NoSimpleType); ok || got != nil {
		t.Fatalf("UsableSimpleType(invalid) = %+v, %v; want nil, false", got, ok)
	}
}

func TestValidWhitespaceRestriction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base WhitespaceMode
		next WhitespaceMode
		want bool
	}{
		{name: "preserve to preserve", base: WhitespacePreserve, next: WhitespacePreserve, want: true},
		{name: "preserve to replace", base: WhitespacePreserve, next: WhitespaceReplace, want: true},
		{name: "preserve to collapse", base: WhitespacePreserve, next: WhitespaceCollapse, want: true},
		{name: "replace to preserve", base: WhitespaceReplace, next: WhitespacePreserve},
		{name: "replace to replace", base: WhitespaceReplace, next: WhitespaceReplace, want: true},
		{name: "replace to collapse", base: WhitespaceReplace, next: WhitespaceCollapse, want: true},
		{name: "collapse to preserve", base: WhitespaceCollapse, next: WhitespacePreserve},
		{name: "collapse to replace", base: WhitespaceCollapse, next: WhitespaceReplace},
		{name: "collapse to collapse", base: WhitespaceCollapse, next: WhitespaceCollapse, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ValidWhitespaceRestriction(tt.base, tt.next); got != tt.want {
				t.Fatalf("ValidWhitespaceRestriction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleTypeReadProjectionHelpers(t *testing.T) {
	t.Parallel()

	shapes := []SimpleTypeReadShape{
		{Primitive: PrimitiveString, Identity: SimpleIdentityNone},
		{Primitive: PrimitiveDecimal, Identity: SimpleIdentityID, Final: DerivationRestriction},
		{Primitive: PrimitiveQName, Identity: SimpleIdentityIDREF, Final: DerivationList},
	}

	primitives := NewSimpleTypePrimitiveReads(shapes)
	if !EqualSimpleTypePrimitiveReadProjection(primitives, shapes) {
		t.Fatalf("NewSimpleTypePrimitiveReads() = %v, want projection for %v", primitives, shapes)
	}
	if got, ok := SimpleTypePrimitiveByID(primitives, 1); !ok || got != PrimitiveDecimal {
		t.Fatalf("SimpleTypePrimitiveByID() = %v, %v; want decimal, true", got, ok)
	}
	if got, ok := SimpleTypePrimitiveByID(primitives, SimpleTypeID(99)); ok || got != 0 {
		t.Fatalf("SimpleTypePrimitiveByID(invalid) = %v, %v; want zero, false", got, ok)
	}
	primitives[1] = PrimitiveBoolean
	if EqualSimpleTypePrimitiveReadProjection(primitives, shapes) {
		t.Fatal("EqualSimpleTypePrimitiveReadProjection() accepted mismatched primitive")
	}
	if EqualSimpleTypePrimitiveReadProjection(primitives[:1], shapes) {
		t.Fatal("EqualSimpleTypePrimitiveReadProjection() accepted mismatched table length")
	}

	identities := NewSimpleTypeIdentityReads(shapes)
	if !EqualSimpleTypeIdentityReadProjection(identities, shapes) {
		t.Fatalf("NewSimpleTypeIdentityReads() = %v, want projection for %v", identities, shapes)
	}
	if got := SimpleTypeIdentityByID(identities, 2); got != SimpleIdentityIDREF {
		t.Fatalf("SimpleTypeIdentityByID() = %v, want IDREF", got)
	}
	if got := SimpleTypeIdentityByID(identities, SimpleTypeID(99)); got != SimpleIdentityNone {
		t.Fatalf("SimpleTypeIdentityByID(invalid) = %v, want none", got)
	}
	identities[2] = SimpleIdentityIDREFList
	if EqualSimpleTypeIdentityReadProjection(identities, shapes) {
		t.Fatal("EqualSimpleTypeIdentityReadProjection() accepted mismatched identity")
	}
	if EqualSimpleTypeIdentityReadProjection(identities[:1], shapes) {
		t.Fatal("EqualSimpleTypeIdentityReadProjection() accepted mismatched table length")
	}

	finals := NewSimpleTypeFinalReads(shapes)
	if !EqualSimpleTypeFinalReadProjection(finals, shapes) {
		t.Fatalf("NewSimpleTypeFinalReads() = %v, want projection for %v", finals, shapes)
	}
	if got, ok := SimpleTypeFinalByID(finals, 1); !ok || got != DerivationRestriction {
		t.Fatalf("SimpleTypeFinalByID() = %v, %v; want restriction, true", got, ok)
	}
	if got, ok := SimpleTypeFinalByID(finals, SimpleTypeID(99)); ok || got != 0 {
		t.Fatalf("SimpleTypeFinalByID(invalid) = %v, %v; want zero, false", got, ok)
	}
	finals[1] = DerivationUnion
	if EqualSimpleTypeFinalReadProjection(finals, shapes) {
		t.Fatal("EqualSimpleTypeFinalReadProjection() accepted mismatched final")
	}
	if EqualSimpleTypeFinalReadProjection(finals[:1], shapes) {
		t.Fatal("EqualSimpleTypeFinalReadProjection() accepted mismatched table length")
	}

	types := []SimpleType{
		{Primitive: PrimitiveString, Identity: SimpleIdentityID},
		{Primitive: PrimitiveDecimal, Identity: SimpleIdentityNone, Final: DerivationRestriction},
	}
	typePrimitives := NewSimpleTypePrimitiveReadsForTypes(types)
	if !EqualSimpleTypePrimitiveReadProjectionForTypes(typePrimitives, types) {
		t.Fatalf("NewSimpleTypePrimitiveReadsForTypes() = %v, want projection for %v", typePrimitives, types)
	}
	typePrimitives[1] = PrimitiveBoolean
	if EqualSimpleTypePrimitiveReadProjectionForTypes(typePrimitives, types) {
		t.Fatal("EqualSimpleTypePrimitiveReadProjectionForTypes() accepted mismatched primitive")
	}
	if err := ValidateSimpleTypePrimitiveReadProjectionForTypes(NewSimpleTypePrimitiveReadsForTypes(types), types); err != nil {
		t.Fatalf("ValidateSimpleTypePrimitiveReadProjectionForTypes() error = %v", err)
	}
	if err := ValidateSimpleTypePrimitiveReadProjectionForTypes(typePrimitives[:1], types); err == nil || err.Error() != "simple type primitive projection count does not match types" {
		t.Fatalf("ValidateSimpleTypePrimitiveReadProjectionForTypes(short) error = %v, want count invariant", err)
	}
	if err := ValidateSimpleTypePrimitiveReadProjectionForTypes(typePrimitives, types); err == nil || err.Error() != "simple type primitive projection does not match type" {
		t.Fatalf("ValidateSimpleTypePrimitiveReadProjectionForTypes(changed) error = %v, want mismatch invariant", err)
	}

	typeIdentities := NewSimpleTypeIdentityReadsForTypes(types)
	if !EqualSimpleTypeIdentityReadProjectionForTypes(typeIdentities, types) {
		t.Fatalf("NewSimpleTypeIdentityReadsForTypes() = %v, want projection for %v", typeIdentities, types)
	}
	typeIdentities[0] = SimpleIdentityNone
	if EqualSimpleTypeIdentityReadProjectionForTypes(typeIdentities, types) {
		t.Fatal("EqualSimpleTypeIdentityReadProjectionForTypes() accepted mismatched identity")
	}
	if err := ValidateSimpleTypeIdentityReadProjectionForTypes(NewSimpleTypeIdentityReadsForTypes(types), types); err != nil {
		t.Fatalf("ValidateSimpleTypeIdentityReadProjectionForTypes() error = %v", err)
	}
	if err := ValidateSimpleTypeIdentityReadProjectionForTypes(typeIdentities[:1], types); err == nil || err.Error() != "simple type identity projection count does not match types" {
		t.Fatalf("ValidateSimpleTypeIdentityReadProjectionForTypes(short) error = %v, want count invariant", err)
	}
	if err := ValidateSimpleTypeIdentityReadProjectionForTypes(typeIdentities, types); err == nil || err.Error() != "simple type identity projection does not match type" {
		t.Fatalf("ValidateSimpleTypeIdentityReadProjectionForTypes(changed) error = %v, want mismatch invariant", err)
	}

	typeFinals := NewSimpleTypeFinalReadsForTypes(types)
	if !EqualSimpleTypeFinalReadProjectionForTypes(typeFinals, types) {
		t.Fatalf("NewSimpleTypeFinalReadsForTypes() = %v, want projection for %v", typeFinals, types)
	}
	typeFinals[1] = DerivationList
	if EqualSimpleTypeFinalReadProjectionForTypes(typeFinals, types) {
		t.Fatal("EqualSimpleTypeFinalReadProjectionForTypes() accepted mismatched final")
	}
	if err := ValidateSimpleTypeFinalReadProjectionForTypes(NewSimpleTypeFinalReadsForTypes(types), types); err != nil {
		t.Fatalf("ValidateSimpleTypeFinalReadProjectionForTypes() error = %v", err)
	}
	if err := ValidateSimpleTypeFinalReadProjectionForTypes(typeFinals[:1], types); err == nil || err.Error() != "simple type final projection count does not match types" {
		t.Fatalf("ValidateSimpleTypeFinalReadProjectionForTypes(short) error = %v, want count invariant", err)
	}
	if err := ValidateSimpleTypeFinalReadProjectionForTypes(typeFinals, types); err == nil || err.Error() != "simple type final projection does not match type" {
		t.Fatalf("ValidateSimpleTypeFinalReadProjectionForTypes(changed) error = %v, want mismatch invariant", err)
	}
}

func TestBuiltinValidationKindValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []BuiltinValidationKind{
		BuiltinValidationNone,
		BuiltinValidationInteger,
		BuiltinValidationName,
		BuiltinValidationNCName,
		BuiltinValidationNMTOKEN,
		BuiltinValidationLanguage,
		BuiltinValidationEntity,
		BuiltinValidationXMLLang,
		BuiltinValidationXMLSpace,
	} {
		if !ValidBuiltinValidationKind(kind) {
			t.Fatalf("ValidBuiltinValidationKind(%d) = false", kind)
		}
	}
	if ValidBuiltinValidationKind(BuiltinValidationKind(99)) {
		t.Fatal("invalid builtin validation kind was accepted")
	}
}

func TestValidateFacetMaskShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		shape   FacetMaskShape
	}{
		{
			name: "valid fixed whitespace",
			shape: FacetMaskShape{
				Actual:  FacetLength | FacetPattern,
				Present: FacetLength | FacetPattern,
				Fixed:   FacetLength | FacetWhiteSpace,
			},
		},
		{
			name: "missing present bit",
			shape: FacetMaskShape{
				Actual:  FacetLength,
				Present: 0,
			},
			wantErr: "simple type facet presence mask does not match actual facets",
		},
		{
			name: "extra present bit",
			shape: FacetMaskShape{
				Actual:  0,
				Present: FacetLength,
			},
			wantErr: "simple type facet presence mask does not match actual facets",
		},
		{
			name: "whiteSpace present",
			shape: FacetMaskShape{
				Present: FacetWhiteSpace,
			},
			wantErr: "simple type facet presence mask cannot set whiteSpace",
		},
		{
			name: "fixed exceeds present",
			shape: FacetMaskShape{
				Actual:  FacetLength,
				Present: FacetLength,
				Fixed:   FacetLength | FacetPattern,
			},
			wantErr: "simple type facet fixed mask exceeds present facets",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFacetMaskShape(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateFacetMaskShape() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateFacetMaskShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleTypeFacetMaskShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		wantErr   string
		shape     FacetMaskShape
		variety   SimpleVariety
		primitive PrimitiveKind
	}{
		{
			name:      "valid atomic string length",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveString,
			shape: FacetMaskShape{
				Actual:  FacetLength | FacetPattern,
				Present: FacetLength | FacetPattern,
			},
		},
		{
			name:      "mask mismatch",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveString,
			shape: FacetMaskShape{
				Actual:  FacetLength,
				Present: 0,
			},
			wantErr: "simple type facet presence mask does not match actual facets",
		},
		{
			name:      "decimal length disallowed",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveDecimal,
			shape: FacetMaskShape{
				Actual:  FacetLength,
				Present: FacetLength,
			},
			wantErr: "simple type stores facet not allowed for type",
		},
		{
			name:      "union order disallowed",
			variety:   SimpleVarietyUnion,
			primitive: PrimitiveString,
			shape: FacetMaskShape{
				Actual:  FacetMinInclusive,
				Present: FacetMinInclusive,
			},
			wantErr: "simple type stores facet not allowed for type",
		},
		{
			name:      "unknown facet bit disallowed",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveString,
			shape: FacetMaskShape{
				Actual:  FacetMask(1 << 15),
				Present: FacetMask(1 << 15),
			},
			wantErr: "simple type stores facet not allowed for type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeFacetMaskShape(tt.variety, tt.primitive, tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeFacetMaskShape() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleTypeFacetMaskShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestFacetAllowedForSimpleType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		variety   SimpleVariety
		primitive PrimitiveKind
		facet     FacetMask
		want      bool
	}{
		{
			name:      "atomic string length",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveString,
			facet:     FacetLength,
			want:      true,
		},
		{
			name:      "atomic string order disallowed",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveString,
			facet:     FacetMinInclusive,
		},
		{
			name:      "atomic decimal digits",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveDecimal,
			facet:     FacetTotalDigits,
			want:      true,
		},
		{
			name:      "atomic decimal length disallowed",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveDecimal,
			facet:     FacetLength,
		},
		{
			name:      "atomic date order",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveDate,
			facet:     FacetMaxExclusive,
			want:      true,
		},
		{
			name:      "atomic boolean common facet",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveBoolean,
			facet:     FacetPattern,
			want:      true,
		},
		{
			name:      "list length",
			variety:   SimpleVarietyList,
			primitive: PrimitiveString,
			facet:     FacetMinLength,
			want:      true,
		},
		{
			name:      "list order disallowed",
			variety:   SimpleVarietyList,
			primitive: PrimitiveString,
			facet:     FacetMinInclusive,
		},
		{
			name:      "list whitespace",
			variety:   SimpleVarietyList,
			primitive: PrimitiveString,
			facet:     FacetWhiteSpace,
			want:      true,
		},
		{
			name:      "union enumeration",
			variety:   SimpleVarietyUnion,
			primitive: PrimitiveString,
			facet:     FacetEnumeration,
			want:      true,
		},
		{
			name:      "union whitespace disallowed",
			variety:   SimpleVarietyUnion,
			primitive: PrimitiveString,
			facet:     FacetWhiteSpace,
		},
		{
			name:      "invalid facet mask",
			variety:   SimpleVarietyAtomic,
			primitive: PrimitiveString,
			facet:     FacetLength | FacetPattern,
		},
		{
			name:      "invalid variety",
			variety:   SimpleVariety(99),
			primitive: PrimitiveString,
			facet:     FacetPattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FacetAllowedForSimpleType(tt.variety, tt.primitive, tt.facet)
			if got != tt.want {
				t.Fatalf("FacetAllowedForSimpleType(%d, %d, %d) = %v, want %v", tt.variety, tt.primitive, tt.facet, got, tt.want)
			}
		})
	}
}

func TestValidateDecimalBoundFacetLiterals(t *testing.T) {
	t.Parallel()

	decimalBound := DecimalBoundFacetLiteral{
		Present:     true,
		ActualValid: true,
		ActualKind:  PrimitiveDecimal,
	}
	valid := DecimalBoundFacetLiteralShape{
		Variety:      SimpleVarietyAtomic,
		Primitive:    PrimitiveDecimal,
		MinInclusive: decimalBound,
		MaxInclusive: decimalBound,
		MinExclusive: decimalBound,
		MaxExclusive: decimalBound,
	}
	tests := []struct {
		name    string
		wantErr string
		shape   DecimalBoundFacetLiteralShape
	}{
		{
			name:  "decimal bounds carry decimal actuals",
			shape: valid,
		},
		{
			name: "missing decimal actual",
			shape: func() DecimalBoundFacetLiteralShape {
				shape := valid
				shape.MinInclusive.ActualValid = false
				return shape
			}(),
			wantErr: "decimal bound facet literal lacks decimal actual value",
		},
		{
			name: "wrong actual kind",
			shape: func() DecimalBoundFacetLiteralShape {
				shape := valid
				shape.MaxExclusive.ActualKind = PrimitiveString
				return shape
			}(),
			wantErr: "decimal bound facet literal lacks decimal actual value",
		},
		{
			name: "absent bound does not need actual",
			shape: DecimalBoundFacetLiteralShape{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
			},
		},
		{
			name: "non-decimal primitive does not need decimal actual",
			shape: DecimalBoundFacetLiteralShape{
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveString,
				MinInclusive: DecimalBoundFacetLiteral{Present: true},
			},
		},
		{
			name: "non-atomic simple type does not need decimal actual",
			shape: DecimalBoundFacetLiteralShape{
				Variety:      SimpleVarietyList,
				Primitive:    PrimitiveDecimal,
				MinInclusive: DecimalBoundFacetLiteral{Present: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDecimalBoundFacetLiterals(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDecimalBoundFacetLiterals() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateDecimalBoundFacetLiterals() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFacetCardinalityShape(t *testing.T) {
	t.Parallel()

	value := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	tests := []struct {
		name    string
		wantErr string
		shape   FacetCardinalityShape
	}{
		{
			name: "valid atomic exact length",
			shape: FacetCardinalityShape{
				Variety:   SimpleVarietyAtomic,
				Length:    value(2),
				MinLength: value(2),
				MaxLength: value(2),
			},
		},
		{
			name: "valid list length with lower minLength",
			shape: FacetCardinalityShape{
				Variety:   SimpleVarietyList,
				Length:    value(2),
				MinLength: value(1),
			},
		},
		{
			name: "non-list length differs from minLength",
			shape: FacetCardinalityShape{
				Variety:   SimpleVarietyAtomic,
				Length:    value(2),
				MinLength: value(1),
			},
			wantErr: "length must equal minLength",
		},
		{
			name: "list length less than minLength",
			shape: FacetCardinalityShape{
				Variety:   SimpleVarietyList,
				Length:    value(1),
				MinLength: value(2),
			},
			wantErr: "length cannot be less than minLength",
		},
		{
			name: "length differs from maxLength",
			shape: FacetCardinalityShape{
				Length:    value(2),
				MaxLength: value(3),
			},
			wantErr: "length must equal maxLength",
		},
		{
			name: "minLength exceeds maxLength",
			shape: FacetCardinalityShape{
				MinLength: value(3),
				MaxLength: value(2),
			},
			wantErr: "minLength cannot exceed maxLength",
		},
		{
			name: "fractionDigits exceeds totalDigits",
			shape: FacetCardinalityShape{
				TotalDigits:    value(2),
				FractionDigits: value(3),
			},
			wantErr: "fractionDigits cannot exceed totalDigits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFacetCardinalityShape(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateFacetCardinalityShape() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateFacetCardinalityShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFacetCardinalityRestriction(t *testing.T) {
	t.Parallel()

	value := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	tests := []struct {
		name    string
		wantErr string
		derived FacetCardinalityShape
		base    FacetCardinalityShape
	}{
		{
			name: "valid equal length",
			derived: FacetCardinalityShape{
				Length: value(2),
			},
			base: FacetCardinalityShape{
				Length: value(2),
			},
		},
		{
			name: "missing base facet is unrestricted",
			derived: FacetCardinalityShape{
				Length: value(2),
			},
		},
		{
			name: "length changes base length",
			derived: FacetCardinalityShape{
				Length: value(3),
			},
			base: FacetCardinalityShape{
				Length: value(2),
			},
			wantErr: "length must equal base length",
		},
		{
			name: "length loses base length",
			base: FacetCardinalityShape{
				Length: value(2),
			},
			wantErr: "length must equal base length",
		},
		{
			name: "minLength tightens base minLength",
			derived: FacetCardinalityShape{
				MinLength: value(4),
			},
			base: FacetCardinalityShape{
				MinLength: value(2),
			},
		},
		{
			name: "minLength loosens base minLength",
			derived: FacetCardinalityShape{
				MinLength: value(1),
			},
			base: FacetCardinalityShape{
				MinLength: value(2),
			},
			wantErr: "minLength cannot be less than base minLength",
		},
		{
			name: "maxLength tightens base maxLength",
			derived: FacetCardinalityShape{
				MaxLength: value(2),
			},
			base: FacetCardinalityShape{
				MaxLength: value(4),
			},
		},
		{
			name: "maxLength loosens base maxLength",
			derived: FacetCardinalityShape{
				MaxLength: value(5),
			},
			base: FacetCardinalityShape{
				MaxLength: value(4),
			},
			wantErr: "maxLength cannot exceed base maxLength",
		},
		{
			name: "totalDigits loosens base totalDigits",
			derived: FacetCardinalityShape{
				TotalDigits: value(5),
			},
			base: FacetCardinalityShape{
				TotalDigits: value(4),
			},
			wantErr: "totalDigits cannot exceed base totalDigits",
		},
		{
			name: "fractionDigits loosens base fractionDigits",
			derived: FacetCardinalityShape{
				FractionDigits: value(3),
			},
			base: FacetCardinalityShape{
				FractionDigits: value(2),
			},
			wantErr: "fractionDigits cannot exceed base fractionDigits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFacetCardinalityRestriction(tt.derived, tt.base)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateFacetCardinalityRestriction() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateFacetCardinalityRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFixedFacetPreservation(t *testing.T) {
	t.Parallel()

	value := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	values := func() FixedFacetValues {
		return FixedFacetValues{
			Length:         value(1),
			MinLength:      value(2),
			MaxLength:      value(3),
			TotalDigits:    value(4),
			FractionDigits: value(5),
			Whitespace:     WhitespaceReplace,
		}
	}
	preservedLiteral := FixedLiteralFacetPreservation{
		BasePresent:    true,
		DerivedPresent: true,
		Equal:          true,
	}
	shape := func(fixed FacetMask, mutate func(*FixedFacetPreservation)) FixedFacetPreservation {
		out := FixedFacetPreservation{
			BaseFixed:    fixed,
			Base:         values(),
			Derived:      values(),
			MinInclusive: preservedLiteral,
			MaxInclusive: preservedLiteral,
			MinExclusive: preservedLiteral,
			MaxExclusive: preservedLiteral,
		}
		if mutate != nil {
			mutate(&out)
		}
		return out
	}
	tests := []struct {
		name    string
		wantErr string
		shape   FixedFacetPreservation
	}{
		{
			name:  "no fixed facets",
			shape: shape(0, nil),
		},
		{
			name:  "preserves all supported fixed facets",
			shape: shape(FacetLength|FacetMinLength|FacetMaxLength|FacetTotalDigits|FacetFractionDigits|FacetWhiteSpace|FacetMinInclusive|FacetMaxInclusive|FacetMinExclusive|FacetMaxExclusive, nil),
		},
		{
			name: "fixed length changes",
			shape: shape(FacetLength, func(s *FixedFacetPreservation) {
				s.Derived.Length = value(9)
			}),
			wantErr: "fixed length facet cannot change",
		},
		{
			name: "fixed minLength changes",
			shape: shape(FacetMinLength, func(s *FixedFacetPreservation) {
				s.Derived.MinLength = value(9)
			}),
			wantErr: "fixed minLength facet cannot change",
		},
		{
			name: "fixed maxLength changes",
			shape: shape(FacetMaxLength, func(s *FixedFacetPreservation) {
				s.Derived.MaxLength = value(9)
			}),
			wantErr: "fixed maxLength facet cannot change",
		},
		{
			name: "fixed totalDigits changes",
			shape: shape(FacetTotalDigits, func(s *FixedFacetPreservation) {
				s.Derived.TotalDigits = value(9)
			}),
			wantErr: "fixed totalDigits facet cannot change",
		},
		{
			name: "fixed fractionDigits changes",
			shape: shape(FacetFractionDigits, func(s *FixedFacetPreservation) {
				s.Derived.FractionDigits = value(9)
			}),
			wantErr: "fixed fractionDigits facet cannot change",
		},
		{
			name: "fixed length missing",
			shape: shape(FacetLength, func(s *FixedFacetPreservation) {
				s.Derived.Length = FacetCardinalityValue{}
			}),
			wantErr: "fixed length facet cannot change",
		},
		{
			name: "fixed whiteSpace changes",
			shape: shape(FacetWhiteSpace, func(s *FixedFacetPreservation) {
				s.Derived.Whitespace = WhitespaceCollapse
			}),
			wantErr: "fixed whiteSpace facet cannot change",
		},
		{
			name: "fixed ordered literal missing",
			shape: shape(FacetMinInclusive, func(s *FixedFacetPreservation) {
				s.MinInclusive = FixedLiteralFacetPreservation{BasePresent: true}
			}),
			wantErr: "fixed minInclusive facet cannot change",
		},
		{
			name: "fixed ordered literal changes",
			shape: shape(FacetMaxExclusive, func(s *FixedFacetPreservation) {
				s.MaxExclusive.Equal = false
			}),
			wantErr: "fixed maxExclusive facet cannot change",
		},
		{
			name:    "fixed pattern is unsupported",
			shape:   shape(FacetPattern, nil),
			wantErr: "fixed facet family cannot be preserved",
		},
		{
			name:    "fixed enumeration is unsupported",
			shape:   shape(FacetEnumeration, nil),
			wantErr: "fixed facet family cannot be preserved",
		},
		{
			name:    "unknown fixed bit is unsupported",
			shape:   shape(FacetMask(1<<15), nil),
			wantErr: "fixed facet family cannot be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFixedFacetPreservation(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateFixedFacetPreservation() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateFixedFacetPreservation() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOrderedFacetStep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		step    OrderedFacetStep
	}{
		{name: "empty"},
		{name: "lower inclusive", step: OrderedFacetStep{MinInclusive: true}},
		{name: "lower exclusive", step: OrderedFacetStep{MinExclusive: true}},
		{name: "upper inclusive", step: OrderedFacetStep{MaxInclusive: true}},
		{name: "upper exclusive", step: OrderedFacetStep{MaxExclusive: true}},
		{
			name: "both lower bounds",
			step: OrderedFacetStep{
				MinInclusive: true,
				MinExclusive: true,
			},
			wantErr: "minInclusive and minExclusive cannot both be specified",
		},
		{
			name: "both upper bounds",
			step: OrderedFacetStep{
				MaxInclusive: true,
				MaxExclusive: true,
			},
			wantErr: "maxInclusive and maxExclusive cannot both be specified",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOrderedFacetStep(tt.step)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOrderedFacetStep() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateOrderedFacetStep() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOrderedFacetBaseRestriction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		shape   OrderedFacetBaseRestriction
	}{
		{
			name: "no ordered facet in step",
		},
		{
			name: "declared ordered facet restricts base",
			shape: OrderedFacetBaseRestriction{
				Step:                 OrderedFacetStep{MinInclusive: true},
				DerivedRestrictsBase: true,
			},
		},
		{
			name: "declared ordered facet loosens base",
			shape: OrderedFacetBaseRestriction{
				Step: OrderedFacetStep{MaxExclusive: true},
			},
			wantErr: "ordered facets cannot loosen base ordered facets",
		},
		{
			name: "multiple declared ordered facets loosen base",
			shape: OrderedFacetBaseRestriction{
				Step: OrderedFacetStep{
					MinInclusive: true,
					MaxInclusive: true,
				},
			},
			wantErr: "ordered facets cannot loosen base ordered facets",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOrderedFacetBaseRestriction(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOrderedFacetBaseRestriction() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateOrderedFacetBaseRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOrderedFacetBounds(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	tests := []struct {
		name    string
		wantErr string
		shape   OrderedFacetBoundsValidation
	}{
		{
			name: "decimal total order accepts lower before upper",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveDecimal,
				Lower:     inclusive,
				Upper:     inclusive,
				Relation:  OrderedFacetLess,
			},
		},
		{
			name: "decimal total order rejects incomparable",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveDecimal,
				Lower:     inclusive,
				Upper:     inclusive,
				Relation:  OrderedFacetIncomparable,
			},
			wantErr: "decimal lower bound cannot exceed upper bound",
		},
		{
			name: "float maps to float diagnostic",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveFloat,
				Lower:     inclusive,
				Upper:     inclusive,
				Relation:  OrderedFacetGreater,
			},
			wantErr: "float lower bound cannot exceed upper bound",
		},
		{
			name: "partial temporal order accepts incomparable",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveDateTime,
				Lower:     inclusive,
				Upper:     inclusive,
				Relation:  OrderedFacetIncomparable,
			},
		},
		{
			name: "partial temporal order rejects equal exclusive bound",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveTime,
				Lower:     exclusive,
				Upper:     inclusive,
				Relation:  OrderedFacetEqual,
			},
			wantErr: "temporal lower bound cannot exceed upper bound",
		},
		{
			name: "g value keeps primitive diagnostic",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveGMonthDay,
				Lower:     inclusive,
				Upper:     inclusive,
				Relation:  OrderedFacetGreater,
			},
			wantErr: "gMonthDay lower bound cannot exceed upper bound",
		},
		{
			name: "unsupported primitive",
			shape: OrderedFacetBoundsValidation{
				Primitive: PrimitiveString,
			},
			wantErr: "primitive does not support ordered facet bounds",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOrderedFacetBounds(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOrderedFacetBounds() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateOrderedFacetBounds() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOrderedFacetBoundRestriction(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	tests := []struct {
		name    string
		wantErr string
		shape   OrderedFacetBoundRestriction
		upper   bool
	}{
		{
			name: "lower tightens",
			shape: OrderedFacetBoundRestriction{
				Facet:    "minInclusive",
				Derived:  inclusive,
				Base:     inclusive,
				Relation: OrderedFacetGreater,
			},
		},
		{
			name: "lower loosens",
			shape: OrderedFacetBoundRestriction{
				Facet:    "minInclusive",
				Derived:  inclusive,
				Base:     exclusive,
				Relation: OrderedFacetEqual,
			},
			wantErr: "minInclusive cannot be less than base lower bound",
		},
		{
			name:  "upper tightens",
			upper: true,
			shape: OrderedFacetBoundRestriction{
				Facet:    "maxExclusive",
				Derived:  exclusive,
				Base:     inclusive,
				Relation: OrderedFacetLess,
			},
		},
		{
			name:  "upper loosens",
			upper: true,
			shape: OrderedFacetBoundRestriction{
				Facet:    "maxInclusive",
				Derived:  inclusive,
				Base:     inclusive,
				Relation: OrderedFacetGreater,
			},
			wantErr: "maxInclusive cannot exceed base upper bound",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var err error
			if tt.upper {
				err = ValidateOrderedFacetUpperRestriction(tt.shape)
			} else {
				err = ValidateOrderedFacetLowerRestriction(tt.shape)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOrderedFacetBoundRestriction() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateOrderedFacetBoundRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestOrderedFacetLowerRestricts(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	invalid := OrderedFacetBound{Kind: OrderedFacetBoundKind(99)}
	absent := OrderedFacetBound{}
	tests := []struct {
		name     string
		derived  OrderedFacetBound
		base     OrderedFacetBound
		relation OrderedFacetRelation
		want     bool
	}{
		{name: "absent base", derived: absent, base: absent, relation: OrderedFacetIncomparable, want: true},
		{name: "missing derived", derived: absent, base: inclusive, relation: OrderedFacetGreater},
		{name: "greater derived bound", derived: inclusive, base: inclusive, relation: OrderedFacetGreater, want: true},
		{name: "less derived bound", derived: inclusive, base: inclusive, relation: OrderedFacetLess},
		{name: "equal inclusive preserves inclusive", derived: inclusive, base: inclusive, relation: OrderedFacetEqual, want: true},
		{name: "equal exclusive tightens inclusive", derived: exclusive, base: inclusive, relation: OrderedFacetEqual, want: true},
		{name: "equal inclusive loosens exclusive", derived: inclusive, base: exclusive, relation: OrderedFacetEqual},
		{name: "equal exclusive preserves exclusive", derived: exclusive, base: exclusive, relation: OrderedFacetEqual, want: true},
		{name: "incomparable", derived: inclusive, base: inclusive, relation: OrderedFacetIncomparable},
		{name: "invalid relation", derived: inclusive, base: inclusive, relation: OrderedFacetRelation(99)},
		{name: "invalid derived kind", derived: invalid, base: inclusive, relation: OrderedFacetGreater},
		{name: "invalid base kind", derived: inclusive, base: invalid, relation: OrderedFacetGreater},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := OrderedFacetLowerRestricts(tt.derived, tt.base, tt.relation)
			if got != tt.want {
				t.Fatalf("OrderedFacetLowerRestricts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderedFacetUpperRestricts(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	invalid := OrderedFacetBound{Kind: OrderedFacetBoundKind(99)}
	absent := OrderedFacetBound{}
	tests := []struct {
		name     string
		derived  OrderedFacetBound
		base     OrderedFacetBound
		relation OrderedFacetRelation
		want     bool
	}{
		{name: "absent base", derived: absent, base: absent, relation: OrderedFacetIncomparable, want: true},
		{name: "missing derived", derived: absent, base: inclusive, relation: OrderedFacetLess},
		{name: "less derived bound", derived: inclusive, base: inclusive, relation: OrderedFacetLess, want: true},
		{name: "greater derived bound", derived: inclusive, base: inclusive, relation: OrderedFacetGreater},
		{name: "equal inclusive preserves inclusive", derived: inclusive, base: inclusive, relation: OrderedFacetEqual, want: true},
		{name: "equal exclusive tightens inclusive", derived: exclusive, base: inclusive, relation: OrderedFacetEqual, want: true},
		{name: "equal inclusive loosens exclusive", derived: inclusive, base: exclusive, relation: OrderedFacetEqual},
		{name: "equal exclusive preserves exclusive", derived: exclusive, base: exclusive, relation: OrderedFacetEqual, want: true},
		{name: "incomparable", derived: inclusive, base: inclusive, relation: OrderedFacetIncomparable},
		{name: "invalid relation", derived: inclusive, base: inclusive, relation: OrderedFacetRelation(99)},
		{name: "invalid derived kind", derived: invalid, base: inclusive, relation: OrderedFacetLess},
		{name: "invalid base kind", derived: inclusive, base: invalid, relation: OrderedFacetLess},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := OrderedFacetUpperRestricts(tt.derived, tt.base, tt.relation)
			if got != tt.want {
				t.Fatalf("OrderedFacetUpperRestricts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderedFacetBoundsConsistent(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	invalid := OrderedFacetBound{Kind: OrderedFacetBoundKind(99)}
	absent := OrderedFacetBound{}
	tests := []struct {
		name     string
		lower    OrderedFacetBound
		upper    OrderedFacetBound
		relation OrderedFacetRelation
		want     bool
	}{
		{name: "absent lower", lower: absent, upper: inclusive, relation: OrderedFacetGreater, want: true},
		{name: "absent upper", lower: inclusive, upper: absent, relation: OrderedFacetGreater, want: true},
		{name: "lower less than upper", lower: inclusive, upper: inclusive, relation: OrderedFacetLess, want: true},
		{name: "lower greater than upper", lower: inclusive, upper: inclusive, relation: OrderedFacetGreater},
		{name: "equal inclusive bounds", lower: inclusive, upper: inclusive, relation: OrderedFacetEqual, want: true},
		{name: "equal exclusive lower", lower: exclusive, upper: inclusive, relation: OrderedFacetEqual},
		{name: "equal exclusive upper", lower: inclusive, upper: exclusive, relation: OrderedFacetEqual},
		{name: "incomparable total order", lower: inclusive, upper: inclusive, relation: OrderedFacetIncomparable},
		{name: "invalid relation", lower: inclusive, upper: inclusive, relation: OrderedFacetRelation(99)},
		{name: "invalid lower kind", lower: invalid, upper: inclusive, relation: OrderedFacetLess},
		{name: "invalid upper kind", lower: inclusive, upper: invalid, relation: OrderedFacetLess},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := OrderedFacetBoundsConsistent(tt.lower, tt.upper, tt.relation)
			if got != tt.want {
				t.Fatalf("OrderedFacetBoundsConsistent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPartialOrderedFacetBoundsConsistent(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	invalid := OrderedFacetBound{Kind: OrderedFacetBoundKind(99)}
	tests := []struct {
		name     string
		lower    OrderedFacetBound
		upper    OrderedFacetBound
		relation OrderedFacetRelation
		want     bool
	}{
		{name: "incomparable bounds allowed", lower: inclusive, upper: inclusive, relation: OrderedFacetIncomparable, want: true},
		{name: "lower greater than upper", lower: inclusive, upper: inclusive, relation: OrderedFacetGreater},
		{name: "equal inclusive bounds", lower: inclusive, upper: inclusive, relation: OrderedFacetEqual, want: true},
		{name: "equal exclusive bounds", lower: exclusive, upper: inclusive, relation: OrderedFacetEqual},
		{name: "invalid relation", lower: inclusive, upper: inclusive, relation: OrderedFacetRelation(99)},
		{name: "invalid lower kind", lower: invalid, upper: inclusive, relation: OrderedFacetIncomparable},
		{name: "invalid upper kind", lower: inclusive, upper: invalid, relation: OrderedFacetIncomparable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := PartialOrderedFacetBoundsConsistent(tt.lower, tt.upper, tt.relation)
			if got != tt.want {
				t.Fatalf("PartialOrderedFacetBoundsConsistent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderedFacetBoundAccepts(t *testing.T) {
	t.Parallel()

	inclusive := OrderedFacetBound{Kind: OrderedFacetBoundInclusive}
	exclusive := OrderedFacetBound{Kind: OrderedFacetBoundExclusive}
	absent := OrderedFacetBound{}
	invalid := OrderedFacetBound{Kind: OrderedFacetBoundKind(99)}
	tests := []struct {
		name     string
		bound    OrderedFacetBound
		relation OrderedFacetRelation
		lower    bool
		upper    bool
	}{
		{name: "absent accepts valid relation", bound: absent, relation: OrderedFacetIncomparable, lower: true, upper: true},
		{name: "lower inclusive equal", bound: inclusive, relation: OrderedFacetEqual, lower: true, upper: true},
		{name: "lower inclusive greater", bound: inclusive, relation: OrderedFacetGreater, lower: true},
		{name: "lower inclusive less", bound: inclusive, relation: OrderedFacetLess, upper: true},
		{name: "lower exclusive greater", bound: exclusive, relation: OrderedFacetGreater, lower: true},
		{name: "upper exclusive less", bound: exclusive, relation: OrderedFacetLess, upper: true},
		{name: "exclusive equal rejected", bound: exclusive, relation: OrderedFacetEqual},
		{name: "incomparable bound rejected", bound: inclusive, relation: OrderedFacetIncomparable},
		{name: "invalid relation", bound: inclusive, relation: OrderedFacetRelation(99)},
		{name: "invalid bound", bound: invalid, relation: OrderedFacetEqual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := OrderedFacetLowerBoundAccepts(tt.bound, tt.relation); got != tt.lower {
				t.Fatalf("OrderedFacetLowerBoundAccepts() = %v, want %v", got, tt.lower)
			}
			if got := OrderedFacetUpperBoundAccepts(tt.bound, tt.relation); got != tt.upper {
				t.Fatalf("OrderedFacetUpperBoundAccepts() = %v, want %v", got, tt.upper)
			}
		})
	}
}

func TestValidatePatternFacetShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		groups  []PatternFacetGroup
	}{
		{name: "empty"},
		{
			name: "fast matcher",
			groups: []PatternFacetGroup{{
				Patterns: []PatternFacet{{XSDSource: "[A-Z]", HasFast: true, FastSignature: "upper"}},
			}},
		},
		{
			name: "regexp matcher",
			groups: []PatternFacetGroup{{
				Patterns: []PatternFacet{{XSDSource: "[A-Z]", HasRegexp: true, Regexp: "^(?:[A-Z])$"}},
			}},
		},
		{
			name:    "empty group",
			groups:  []PatternFacetGroup{{}},
			wantErr: "simple type pattern facet group has no patterns",
		},
		{
			name: "pattern without matcher",
			groups: []PatternFacetGroup{{
				Patterns: []PatternFacet{{XSDSource: "[A-Z]"}},
			}},
			wantErr: "simple type pattern facet has invalid matcher",
		},
		{
			name: "pattern with both matchers",
			groups: []PatternFacetGroup{{
				Patterns: []PatternFacet{{XSDSource: "[A-Z]", HasFast: true, HasRegexp: true}},
			}},
			wantErr: "simple type pattern facet has invalid matcher",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePatternFacetShape(tt.groups)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidatePatternFacetShape() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidatePatternFacetShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePatternFacetRestriction(t *testing.T) {
	t.Parallel()

	a := PatternFacet{XSDSource: "[A-Z]", FastSignature: "upper", HasFast: true}
	b := PatternFacet{XSDSource: "[0-9]", FastSignature: "digit", HasFast: true}
	c := PatternFacet{XSDSource: "[a-z]", GoSource: "^(?:[a-z])$", Regexp: "^(?:[a-z])$", HasRegexp: true}
	tests := []struct {
		name    string
		wantErr string
		derived []PatternFacetGroup
		base    []PatternFacetGroup
	}{
		{
			name:    "preserves base groups",
			derived: []PatternFacetGroup{{Patterns: []PatternFacet{a}}, {Patterns: []PatternFacet{b}}},
			base:    []PatternFacetGroup{{Patterns: []PatternFacet{a}}, {Patterns: []PatternFacet{b}}},
		},
		{
			name:    "appends derived group",
			derived: []PatternFacetGroup{{Patterns: []PatternFacet{a}}, {Patterns: []PatternFacet{b}}, {Patterns: []PatternFacet{c}}},
			base:    []PatternFacetGroup{{Patterns: []PatternFacet{a}}, {Patterns: []PatternFacet{b}}},
		},
		{
			name:    "omits base group",
			derived: []PatternFacetGroup{{Patterns: []PatternFacet{a}}},
			base:    []PatternFacetGroup{{Patterns: []PatternFacet{a}}, {Patterns: []PatternFacet{b}}},
			wantErr: "simple type patterns loosen base",
		},
		{
			name:    "reorders base groups",
			derived: []PatternFacetGroup{{Patterns: []PatternFacet{b}}, {Patterns: []PatternFacet{a}}},
			base:    []PatternFacetGroup{{Patterns: []PatternFacet{a}}, {Patterns: []PatternFacet{b}}},
			wantErr: "simple type patterns loosen base",
		},
		{
			name:    "rewrites fast matcher",
			derived: []PatternFacetGroup{{Patterns: []PatternFacet{{XSDSource: "[A-Z]", FastSignature: "changed", HasFast: true}}}},
			base:    []PatternFacetGroup{{Patterns: []PatternFacet{a}}},
			wantErr: "simple type patterns loosen base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePatternFacetRestriction(tt.derived, tt.base)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidatePatternFacetRestriction() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidatePatternFacetRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEnumerationFacetRestriction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		shape   EnumerationFacetRestriction
	}{
		{
			name: "base has no enumeration",
			shape: EnumerationFacetRestriction{
				DerivedMatchesBase: []bool{false},
			},
		},
		{
			name: "derived narrows base",
			shape: EnumerationFacetRestriction{
				BaseCount:          2,
				DerivedMatchesBase: []bool{true},
			},
		},
		{
			name: "derived omits enumeration",
			shape: EnumerationFacetRestriction{
				BaseCount: 1,
			},
			wantErr: "simple type enumeration loosens base",
		},
		{
			name: "derived literal outside base",
			shape: EnumerationFacetRestriction{
				BaseCount:          1,
				DerivedMatchesBase: []bool{true, false},
			},
			wantErr: "simple type enumeration loosens base",
		},
		{
			name: "invalid base count",
			shape: EnumerationFacetRestriction{
				BaseCount: -1,
			},
			wantErr: "simple type enumeration shape is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEnumerationFacetRestriction(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateEnumerationFacetRestriction() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateEnumerationFacetRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleFastPath(t *testing.T) {
	t.Parallel()

	valid := SimpleFastPathValidation{
		FractionDigits:           FacetCardinalityValue{Value: 0, Present: true},
		Stored:                   SimpleFastInt,
		Variety:                  SimpleVarietyAtomic,
		Primitive:                PrimitiveDecimal,
		Builtin:                  BuiltinValidationInteger,
		Whitespace:               WhitespaceCollapse,
		MinInclusiveMatchesInt32: true,
		MaxInclusiveMatchesInt32: true,
	}
	tests := []struct {
		name    string
		wantErr string
		shape   SimpleFastPathValidation
		want    SimpleFastKind
	}{
		{
			name:  "int fast path",
			shape: valid,
			want:  SimpleFastInt,
		},
		{
			name: "regular decimal has no fast path",
			shape: func() SimpleFastPathValidation {
				shape := valid
				shape.Stored = SimpleFastNone
				shape.Builtin = BuiltinValidationNone
				return shape
			}(),
			want: SimpleFastNone,
		},
		{
			name: "enumeration disables fast path",
			shape: func() SimpleFastPathValidation {
				shape := valid
				shape.Stored = SimpleFastNone
				shape.EnumerationSize = 1
				return shape
			}(),
			want: SimpleFastNone,
		},
		{
			name: "missing int bound disables fast path",
			shape: func() SimpleFastPathValidation {
				shape := valid
				shape.Stored = SimpleFastNone
				shape.MinInclusiveMatchesInt32 = false
				return shape
			}(),
			want: SimpleFastNone,
		},
		{
			name: "stored fast path drift",
			shape: func() SimpleFastPathValidation {
				shape := valid
				shape.Stored = SimpleFastNone
				return shape
			}(),
			want:    SimpleFastInt,
			wantErr: "simple type fast path does not match facets",
		},
		{
			name: "invalid stored fast path",
			shape: func() SimpleFastPathValidation {
				shape := valid
				shape.Stored = SimpleFastKind(99)
				return shape
			}(),
			want:    SimpleFastInt,
			wantErr: "simple type fast path is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := DeriveSimpleFastPath(tt.shape); got != tt.want {
				t.Fatalf("DeriveSimpleFastPath() = %v, want %v", got, tt.want)
			}
			err := ValidateSimpleFastPath(tt.shape)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleFastPath() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleFastPath() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleValueRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape SimpleValueRouteShape
		want  SimpleValueRouteAction
	}{
		{
			name: "no simple type is untyped",
			shape: SimpleValueRouteShape{
				Type: NoSimpleType,
			},
			want: SimpleValueRouteUntyped,
		},
		{
			name: "missing simple type",
			shape: SimpleValueRouteShape{
				Type: SimpleTypeID(1),
			},
			want: SimpleValueRouteMissing,
		},
		{
			name: "atomic simple type",
			shape: SimpleValueRouteShape{
				Type:    SimpleTypeID(1),
				Variety: SimpleVarietyAtomic,
				Known:   true,
			},
			want: SimpleValueRouteAtomic,
		},
		{
			name: "list simple type",
			shape: SimpleValueRouteShape{
				Type:    SimpleTypeID(1),
				Variety: SimpleVarietyList,
				Known:   true,
			},
			want: SimpleValueRouteList,
		},
		{
			name: "union simple type",
			shape: SimpleValueRouteShape{
				Type:    SimpleTypeID(1),
				Variety: SimpleVarietyUnion,
				Known:   true,
			},
			want: SimpleValueRouteUnion,
		},
		{
			name: "invalid simple type variety",
			shape: SimpleValueRouteShape{
				Type:    SimpleTypeID(1),
				Variety: SimpleVariety(99),
				Known:   true,
			},
			want: SimpleValueRouteInvalid,
		},
		{
			name: "no simple type ignores table presence",
			shape: SimpleValueRouteShape{
				Type:    NoSimpleType,
				Variety: SimpleVarietyAtomic,
				Known:   true,
			},
			want: SimpleValueRouteUntyped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueRoute(tt.shape); got != tt.want {
				t.Fatalf("SimpleValueRoute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleValueBypass(t *testing.T) {
	t.Parallel()

	base := SimpleValueBypassShape{
		Variety:  SimpleVarietyAtomic,
		Builtin:  BuiltinValidationNone,
		Identity: SimpleIdentityNone,
	}
	tests := []struct {
		name  string
		shape SimpleValueBypassShape
		want  SimpleValueBypassAction
	}{
		{
			name: "string accept can produce requested projections",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveString
				shape.Needs = SimpleNeedCanonical | SimpleNeedIdentity
				return shape
			}(),
			want: SimpleValueBypassAcceptString,
		},
		{
			name: "non-atomic cannot bypass",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Variety = SimpleVarietyList
				shape.Primitive = PrimitiveString
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "identity disables string accept",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveString
				shape.Identity = SimpleIdentityID
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "int fast path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDecimal
				shape.Builtin = BuiltinValidationInteger
				shape.Fast = SimpleFastInt
				return shape
			}(),
			want: SimpleValueBypassValidateInt,
		},
		{
			name: "needs disable int no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDecimal
				shape.Builtin = BuiltinValidationInteger
				shape.Fast = SimpleFastInt
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "decimal no-output allows decimal facets",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDecimal
				shape.Facets = FacetTotalDigits | FacetFractionDigits
				return shape
			}(),
			want: SimpleValueBypassValidateDecimal,
		},
		{
			name: "decimal pattern requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDecimal
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "string patterns no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveString
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleValueBypassValidateStringPatterns,
		},
		{
			name: "string enumeration no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveString
				shape.Facets = FacetEnumeration
				return shape
			}(),
			want: SimpleValueBypassValidateStringEnumeration,
		},
		{
			name: "mixed string facets require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveString
				shape.Facets = FacetPattern | FacetEnumeration
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "anyURI no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveAnyURI
				return shape
			}(),
			want: SimpleValueBypassValidateAnyURI,
		},
		{
			name: "anyURI facets require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveAnyURI
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "anyURI needs require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveAnyURI
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "hexBinary no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveHexBinary
				return shape
			}(),
			want: SimpleValueBypassValidateHexBinary,
		},
		{
			name: "hexBinary length facet requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveHexBinary
				shape.Facets = FacetLength
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "hexBinary pattern facet requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveHexBinary
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "hexBinary needs require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveHexBinary
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "hexBinary identity requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveHexBinary
				shape.Identity = SimpleIdentityID
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "base64Binary no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveBase64Binary
				return shape
			}(),
			want: SimpleValueBypassValidateBase64Binary,
		},
		{
			name: "base64Binary length facet requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveBase64Binary
				shape.Facets = FacetLength
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "base64Binary enumeration facet requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveBase64Binary
				shape.Facets = FacetEnumeration
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "base64Binary needs require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveBase64Binary
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "float no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveFloat
				return shape
			}(),
			want: SimpleValueBypassValidateFloat,
		},
		{
			name: "double no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDouble
				return shape
			}(),
			want: SimpleValueBypassValidateFloat,
		},
		{
			name: "float facets require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveFloat
				shape.Facets = FacetMinInclusive
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "double needs require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDouble
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "float identity requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveFloat
				shape.Identity = SimpleIdentityID
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "duration no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDuration
				return shape
			}(),
			want: SimpleValueBypassValidateDuration,
		},
		{
			name: "duration bounds require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDuration
				shape.Facets = FacetMinInclusive
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "duration pattern requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDuration
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "duration enumeration requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDuration
				shape.Facets = FacetEnumeration
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "duration needs require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDuration
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "boolean no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveBoolean
				return shape
			}(),
			want: SimpleValueBypassValidateBoolean,
		},
		{
			name: "dateTime no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDateTime
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "time no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveTime
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "gYearMonth no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGYearMonth
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "gYear no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGYear
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "gMonthDay no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGMonthDay
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "gDay no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGDay
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "gMonth no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGMonth
				return shape
			}(),
			want: SimpleValueBypassValidateTemporal,
		},
		{
			name: "temporal bounds require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDateTime
				shape.Facets = FacetMinInclusive
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "temporal pattern requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveTime
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "temporal enumeration requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGYear
				shape.Facets = FacetEnumeration
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "temporal needs require full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveGMonth
				shape.Needs = SimpleNeedCanonical
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
		{
			name: "date no-output",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveDate
				return shape
			}(),
			want: SimpleValueBypassValidateDate,
		},
		{
			name: "builtin validation requires full path",
			shape: func() SimpleValueBypassShape {
				shape := base
				shape.Primitive = PrimitiveString
				shape.Builtin = BuiltinValidationName
				return shape
			}(),
			want: SimpleValueBypassNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueBypass(tt.shape); got != tt.want {
				t.Fatalf("SimpleValueBypass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleFixedStringFastPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape SimpleFixedStringFastPathShape
		want  bool
	}{
		{
			name: "fixed preserve string accept",
			shape: SimpleFixedStringFastPathShape{
				Bypass:     SimpleValueBypassAcceptString,
				Whitespace: WhitespacePreserve,
				HasFixed:   true,
			},
			want: true,
		},
		{
			name: "missing fixed value",
			shape: SimpleFixedStringFastPathShape{
				Bypass:     SimpleValueBypassAcceptString,
				Whitespace: WhitespacePreserve,
			},
		},
		{
			name: "normalized whitespace requires full validation",
			shape: SimpleFixedStringFastPathShape{
				Bypass:     SimpleValueBypassAcceptString,
				Whitespace: WhitespaceCollapse,
				HasFixed:   true,
			},
		},
		{
			name: "non-accept bypass requires full validation",
			shape: SimpleFixedStringFastPathShape{
				Bypass:     SimpleValueBypassValidateStringEnumeration,
				Whitespace: WhitespacePreserve,
				HasFixed:   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleFixedStringFastPath(tt.shape); got != tt.want {
				t.Fatalf("SimpleFixedStringFastPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleFixedStringFastPathForType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typ      SimpleValueType
		hasFixed bool
		want     bool
	}{
		{
			name: "fixed preserve string accept",
			typ: SimpleValueType{
				Variety:    SimpleVarietyAtomic,
				Primitive:  PrimitiveString,
				Whitespace: WhitespacePreserve,
			},
			hasFixed: true,
			want:     true,
		},
		{
			name: "missing fixed value",
			typ: SimpleValueType{
				Variety:    SimpleVarietyAtomic,
				Primitive:  PrimitiveString,
				Whitespace: WhitespacePreserve,
			},
		},
		{
			name: "collapse whitespace",
			typ: SimpleValueType{
				Variety:    SimpleVarietyAtomic,
				Primitive:  PrimitiveString,
				Whitespace: WhitespaceCollapse,
			},
			hasFixed: true,
		},
		{
			name: "identity",
			typ: SimpleValueType{
				Variety:    SimpleVarietyAtomic,
				Primitive:  PrimitiveString,
				Whitespace: WhitespacePreserve,
				Identity:   SimpleIdentityID,
			},
			hasFixed: true,
		},
		{
			name: "list",
			typ: SimpleValueType{
				Variety:    SimpleVarietyList,
				Primitive:  PrimitiveString,
				Whitespace: WhitespacePreserve,
			},
			hasFixed: true,
		},
		{
			name: "union",
			typ: SimpleValueType{
				Variety:    SimpleVarietyUnion,
				Primitive:  PrimitiveString,
				Whitespace: WhitespacePreserve,
			},
			hasFixed: true,
		},
		{
			name: "non string",
			typ: SimpleValueType{
				Variety:    SimpleVarietyAtomic,
				Primitive:  PrimitiveBoolean,
				Whitespace: WhitespacePreserve,
			},
			hasFixed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SimpleFixedStringFastPathForType(SimpleFixedStringTypeShape{
				Type:     tt.typ,
				HasFixed: tt.hasFixed,
			})
			if got != tt.want {
				t.Fatalf("SimpleFixedStringFastPathForType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleRawAtomicFastPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape SimpleRawAtomicFastPathShape
		want  SimpleRawAtomicFastPathAction
	}{
		{
			name: "none requires full path",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassNone,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "accept string ignores raw whitespace facts",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassAcceptString,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathAccept,
		},
		{
			name: "string patterns require normalized raw bytes",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:              SimpleValueBypassValidateStringPatterns,
				RawEqualsNormalized: true,
			},
			want: SimpleRawAtomicFastPathValidateStringPatterns,
		},
		{
			name: "string patterns reject normalized mismatch",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateStringPatterns,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "string enumeration requires normalized raw bytes",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:              SimpleValueBypassValidateStringEnumeration,
				RawEqualsNormalized: true,
			},
			want: SimpleRawAtomicFastPathValidateStringEnumeration,
		},
		{
			name: "string enumeration rejects normalized mismatch",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateStringEnumeration,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "int raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateInt,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "int raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateInt,
			},
			want: SimpleRawAtomicFastPathValidateInt,
		},
		{
			name: "decimal raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateDecimal,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "decimal raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateDecimal,
			},
			want: SimpleRawAtomicFastPathValidateDecimal,
		},
		{
			name: "anyURI raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateAnyURI,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "anyURI raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateAnyURI,
			},
			want: SimpleRawAtomicFastPathValidateAnyURI,
		},
		{
			name: "hexBinary raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateHexBinary,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "hexBinary raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateHexBinary,
			},
			want: SimpleRawAtomicFastPathValidateHexBinary,
		},
		{
			name: "base64Binary raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateBase64Binary,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "base64Binary raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateBase64Binary,
			},
			want: SimpleRawAtomicFastPathValidateBase64Binary,
		},
		{
			name: "float raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateFloat,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "float raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateFloat,
			},
			want: SimpleRawAtomicFastPathValidateFloat,
		},
		{
			name: "duration raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateDuration,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "duration raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateDuration,
			},
			want: SimpleRawAtomicFastPathValidateDuration,
		},
		{
			name: "boolean raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateBoolean,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "boolean raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateBoolean,
			},
			want: SimpleRawAtomicFastPathValidateBoolean,
		},
		{
			name: "temporal raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateTemporal,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "temporal raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateTemporal,
			},
			want: SimpleRawAtomicFastPathValidateTemporal,
		},
		{
			name: "date raw path rejects whitespace",
			shape: SimpleRawAtomicFastPathShape{
				Bypass:        SimpleValueBypassValidateDate,
				HasWhitespace: true,
			},
			want: SimpleRawAtomicFastPathNone,
		},
		{
			name: "date raw path accepts no-whitespace input",
			shape: SimpleRawAtomicFastPathShape{
				Bypass: SimpleValueBypassValidateDate,
			},
			want: SimpleRawAtomicFastPathValidateDate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleRawAtomicFastPath(tt.shape); got != tt.want {
				t.Fatalf("SimpleRawAtomicFastPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSimpleTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantErr    string
		final      DerivationMask
		derivation DerivationMask
	}{
		{
			name:       "restriction allowed",
			derivation: DerivationRestriction,
		},
		{
			name:       "list allowed",
			derivation: DerivationList,
		},
		{
			name:       "union allowed",
			derivation: DerivationUnion,
		},
		{
			name:       "restriction blocked",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			wantErr:    "simple type final blocks restriction",
		},
		{
			name:       "list blocked",
			final:      DerivationList | DerivationUnion,
			derivation: DerivationList,
			wantErr:    "simple type final blocks list",
		},
		{
			name:       "union blocked",
			final:      DerivationUnion,
			derivation: DerivationUnion,
			wantErr:    "simple type final blocks union",
		},
		{
			name:       "invalid final mask",
			final:      DerivationExtension,
			derivation: DerivationRestriction,
			wantErr:    "simple type final mask is invalid",
		},
		{
			name:       "invalid derivation",
			derivation: DerivationExtension,
			wantErr:    "simple type final derivation is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeFinalAllows(tt.final, tt.derivation)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeFinalAllows() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleTypeFinalAllows() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleTypeRuntime(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(8, []string{EmptyNamespaceURI}, []ExpandedName{{Local: "type"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, ok := names.LookupQName("", "type")
	if !ok {
		t.Fatal("missing type QName")
	}
	limits := SimpleTypeRefLimits{SimpleTypeCount: 3}
	valid := SimpleTypeValidation{
		Name:       name,
		Base:       NoSimpleType,
		ListItem:   NoSimpleType,
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveString,
		Whitespace: WhitespacePreserve,
		Builtin:    BuiltinValidationNone,
	}
	tests := []struct {
		name    string
		wantErr string
		st      SimpleTypeValidation
	}{
		{
			name: "valid atomic",
			st:   valid,
		},
		{
			name: "valid list",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyList
				st.ListItem = 0
				return st
			}(),
		},
		{
			name: "valid union",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyUnion
				st.Union = []SimpleTypeID{0, 1}
				return st
			}(),
		},
		{
			name: "invalid name",
			st: func() SimpleTypeValidation {
				st := valid
				st.Name = QName{Namespace: 99, Local: 99}
				return st
			}(),
			wantErr: "simple type references invalid name",
		},
		{
			name: "invalid base",
			st: func() SimpleTypeValidation {
				st := valid
				st.Base = 3
				return st
			}(),
			wantErr: "simple type references invalid base",
		},
		{
			name: "invalid primitive",
			st: func() SimpleTypeValidation {
				st := valid
				st.Primitive = PrimitiveKind(99)
				return st
			}(),
			wantErr: "simple type has invalid primitive",
		},
		{
			name: "invalid whitespace",
			st: func() SimpleTypeValidation {
				st := valid
				st.Whitespace = WhitespaceMode(99)
				return st
			}(),
			wantErr: "simple type has invalid whitespace mode",
		},
		{
			name: "invalid builtin",
			st: func() SimpleTypeValidation {
				st := valid
				st.Builtin = BuiltinValidationKind(99)
				return st
			}(),
			wantErr: "simple type has invalid builtin validation kind",
		},
		{
			name: "invalid final",
			st: func() SimpleTypeValidation {
				st := valid
				st.Final = DerivationExtension
				return st
			}(),
			wantErr: "simple type final mask contains invalid derivation",
		},
		{
			name: "atomic list item",
			st: func() SimpleTypeValidation {
				st := valid
				st.ListItem = 0
				return st
			}(),
			wantErr: "atomic simple type stores list item",
		},
		{
			name: "atomic union members",
			st: func() SimpleTypeValidation {
				st := valid
				st.Union = []SimpleTypeID{0}
				return st
			}(),
			wantErr: "atomic simple type stores union members",
		},
		{
			name: "list invalid item",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyList
				st.ListItem = 3
				return st
			}(),
			wantErr: "list simple type references invalid list item",
		},
		{
			name: "list union members",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyList
				st.ListItem = 0
				st.Union = []SimpleTypeID{1}
				return st
			}(),
			wantErr: "list simple type stores union members",
		},
		{
			name: "union list item",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyUnion
				st.ListItem = 0
				st.Union = []SimpleTypeID{1}
				return st
			}(),
			wantErr: "union simple type stores list item",
		},
		{
			name: "union no members",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyUnion
				return st
			}(),
			wantErr: "union simple type has no members",
		},
		{
			name: "union invalid member",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVarietyUnion
				st.Union = []SimpleTypeID{3}
				return st
			}(),
			wantErr: "simple type references invalid union member",
		},
		{
			name: "invalid variety",
			st: func() SimpleTypeValidation {
				st := valid
				st.Variety = SimpleVariety(99)
				return st
			}(),
			wantErr: "simple type has invalid variety",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeRuntime(&names, tt.st, limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleTypeRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleTypeRestrictionRuntime(t *testing.T) {
	t.Parallel()

	valid := SimpleTypeRestrictionValidation{
		Union:      []SimpleTypeID{1, 2},
		ListItem:   NoSimpleType,
		Variety:    SimpleVarietyUnion,
		Primitive:  PrimitiveString,
		Builtin:    BuiltinValidationNone,
		Whitespace: WhitespaceCollapse,
	}
	tests := []struct {
		name    string
		wantErr string
		derived SimpleTypeRestrictionValidation
		base    SimpleTypeRestrictionValidation
	}{
		{
			name:    "valid same semantic fields",
			derived: valid,
			base:    valid,
		},
		{
			name: "valid tighter whitespace",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Whitespace = WhitespaceCollapse
				return st
			}(),
			base: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Whitespace = WhitespaceReplace
				return st
			}(),
		},
		{
			name: "changed variety",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Variety = SimpleVarietyAtomic
				st.Union = nil
				return st
			}(),
			base:    valid,
			wantErr: "simple type semantic fields do not match base restriction",
		},
		{
			name: "changed primitive",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Primitive = PrimitiveBoolean
				return st
			}(),
			base:    valid,
			wantErr: "simple type semantic fields do not match base restriction",
		},
		{
			name: "changed builtin",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Builtin = BuiltinValidationName
				return st
			}(),
			base:    valid,
			wantErr: "simple type semantic fields do not match base restriction",
		},
		{
			name: "changed list item",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.ListItem = 1
				return st
			}(),
			base:    valid,
			wantErr: "simple type semantic fields do not match base restriction",
		},
		{
			name: "changed union order",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Union = []SimpleTypeID{2, 1}
				return st
			}(),
			base:    valid,
			wantErr: "simple type semantic fields do not match base restriction",
		},
		{
			name: "changed union length",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Union = []SimpleTypeID{1}
				return st
			}(),
			base:    valid,
			wantErr: "simple type semantic fields do not match base restriction",
		},
		{
			name: "looser whitespace",
			derived: func() SimpleTypeRestrictionValidation {
				st := valid
				st.Whitespace = WhitespaceReplace
				return st
			}(),
			base:    valid,
			wantErr: "simple type whitespace loosens base restriction",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeRestrictionRuntime(tt.derived, tt.base)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeRestrictionRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleTypeRestrictionRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleTypeRestrictionRequired(t *testing.T) {
	t.Parallel()

	builtins := BuiltinIDs{AnySimpleType: 0}
	var firstUserType SimpleTypeID
	for range BuiltinSimpleTypeCount() {
		firstUserType++
	}
	tests := []struct {
		name string
		id   SimpleTypeID
		base SimpleTypeID
		want bool
	}{
		{name: "no base", id: firstUserType, base: NoSimpleType},
		{name: "anySimple base", id: firstUserType, base: builtins.AnySimpleType},
		{name: "builtin declaration", id: firstUserType - 1, base: 1},
		{name: "first user restriction", id: firstUserType, base: 1, want: true},
		{name: "later user restriction", id: firstUserType + 1, base: firstUserType, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleTypeRestrictionRequired(tt.id, tt.base, builtins); got != tt.want {
				t.Fatalf("SimpleTypeRestrictionRequired(%d, %d, builtins) = %v, want %v", tt.id, tt.base, got, tt.want)
			}
		})
	}
}

type simpleIdentityRuntimeStub map[SimpleTypeID]SimpleIdentityKind

func (s simpleIdentityRuntimeStub) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	identity, ok := s[id]
	return identity, ok
}

func TestDerivedSimpleIdentity(t *testing.T) {
	t.Parallel()

	rt := simpleIdentityRuntimeStub{
		1: SimpleIdentityID,
		2: SimpleIdentityIDREF,
	}
	tests := []struct {
		name string
		node SimpleTypeIdentityNode
		want SimpleIdentityKind
	}{
		{
			name: "atomic inherits base identity",
			node: SimpleTypeIdentityNode{Base: 1, Variety: SimpleVarietyAtomic},
			want: SimpleIdentityID,
		},
		{
			name: "list of IDREF becomes IDREF list",
			node: SimpleTypeIdentityNode{ListItem: 2, Variety: SimpleVarietyList},
			want: SimpleIdentityIDREFList,
		},
		{
			name: "list of ID does not become ID",
			node: SimpleTypeIdentityNode{ListItem: 1, Variety: SimpleVarietyList},
			want: SimpleIdentityNone,
		},
		{
			name: "union has no identity",
			node: SimpleTypeIdentityNode{Variety: SimpleVarietyUnion},
			want: SimpleIdentityNone,
		},
		{
			name: "missing base has no identity",
			node: SimpleTypeIdentityNode{Base: 99, Variety: SimpleVarietyAtomic},
			want: SimpleIdentityNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := DerivedSimpleIdentity(rt, tt.node); got != tt.want {
				t.Fatalf("DerivedSimpleIdentity() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExpectedSimpleIdentity(t *testing.T) {
	t.Parallel()

	builtins := BuiltinIDs{ID: 10, IDREF: 11}
	rt := simpleIdentityRuntimeStub{
		1: SimpleIdentityIDREF,
	}
	if got := ExpectedSimpleIdentity(rt, builtins, builtins.ID, SimpleTypeIdentityNode{}); got != SimpleIdentityID {
		t.Fatalf("ExpectedSimpleIdentity(ID) = %d, want ID", got)
	}
	if got := ExpectedSimpleIdentity(rt, builtins, builtins.IDREF, SimpleTypeIdentityNode{}); got != SimpleIdentityIDREF {
		t.Fatalf("ExpectedSimpleIdentity(IDREF) = %d, want IDREF", got)
	}
	node := SimpleTypeIdentityNode{Base: 1, Variety: SimpleVarietyAtomic}
	if got := ExpectedSimpleIdentity(rt, builtins, 12, node); got != SimpleIdentityIDREF {
		t.Fatalf("ExpectedSimpleIdentity(derived) = %d, want IDREF", got)
	}
}

func TestValidateSimpleTypeIdentity(t *testing.T) {
	t.Parallel()

	builtins := BuiltinIDs{ID: 10, IDREF: 11}
	rt := simpleIdentityRuntimeStub{
		1: SimpleIdentityIDREF,
		2: SimpleIdentityID,
	}
	tests := []struct {
		name    string
		wantErr string
		node    SimpleTypeIdentityNode
		id      SimpleTypeID
		stored  SimpleIdentityKind
	}{
		{
			name:   "builtin ID",
			id:     builtins.ID,
			stored: SimpleIdentityID,
		},
		{
			name:   "builtin IDREF",
			id:     builtins.IDREF,
			stored: SimpleIdentityIDREF,
		},
		{
			name:   "atomic inherits base",
			id:     12,
			node:   SimpleTypeIdentityNode{Base: 1, Variety: SimpleVarietyAtomic},
			stored: SimpleIdentityIDREF,
		},
		{
			name:   "list of IDREF",
			id:     13,
			node:   SimpleTypeIdentityNode{ListItem: 1, Variety: SimpleVarietyList},
			stored: SimpleIdentityIDREFList,
		},
		{
			name:    "mismatch",
			id:      14,
			node:    SimpleTypeIdentityNode{Base: 2, Variety: SimpleVarietyAtomic},
			stored:  SimpleIdentityNone,
			wantErr: "simple type identity does not match derivation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeIdentity(rt, builtins, tt.id, tt.node, tt.stored)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeIdentity() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleTypeIdentity() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleTypeIsID(t *testing.T) {
	t.Parallel()

	rt := simpleIdentityRuntimeStub{
		1: SimpleIdentityID,
		2: SimpleIdentityIDREF,
	}
	if !SimpleTypeIsID(rt, 1) {
		t.Fatal("ID simple type was not recognized")
	}
	if SimpleTypeIsID(rt, 2) {
		t.Fatal("IDREF simple type was recognized as ID")
	}
	if SimpleTypeIsID(rt, 99) {
		t.Fatal("missing simple type was recognized as ID")
	}
}

func TestSimpleIdentityKey(t *testing.T) {
	t.Parallel()

	if got, want := SimpleIdentityKey(PrimitiveString, "abc"), string([]byte{byte(PrimitiveString), 0x1e})+"abc"; got != want {
		t.Fatalf("SimpleIdentityKey() = %q, want %q", got, want)
	}
	if got := SimpleIdentityKey(PrimitiveBoolean, "true"); got == SimpleIdentityKey(PrimitiveString, "true") {
		t.Fatal("SimpleIdentityKey() did not distinguish primitive kind")
	}
	if got, want := UntypedSimpleIdentityKey("abc"), string([]byte{0xff, 0x1e})+"abc"; got != want {
		t.Fatalf("UntypedSimpleIdentityKey() = %q, want %q", got, want)
	}
	if got := UntypedSimpleIdentityKey("true"); got == SimpleIdentityKey(PrimitiveString, "true") {
		t.Fatal("UntypedSimpleIdentityKey() did not distinguish untyped values")
	}
}

func TestSimpleRawListFastPath(t *testing.T) {
	t.Parallel()

	valid := SimpleRawListFastPathShape{
		ListIdentity: SimpleIdentityNone,
		ItemKnown:    true,
		ItemVariety:  SimpleVarietyAtomic,
		ItemBuiltin:  BuiltinValidationNMTOKEN,
		ItemIdentity: SimpleIdentityNone,
	}
	tests := []struct {
		name  string
		shape SimpleRawListFastPathShape
		want  SimpleRawListFastPathAction
	}{
		{
			name:  "NMTOKEN list",
			shape: valid,
			want:  SimpleRawListFastPathValidateNMTOKENList,
		},
		{
			name: "list facets require full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ListFacets = FacetLength
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
		{
			name: "list identity requires full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ListIdentity = SimpleIdentityIDREFList
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
		{
			name: "missing item requires full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ItemKnown = false
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
		{
			name: "non-atomic item requires full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ItemVariety = SimpleVarietyList
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
		{
			name: "non-NMTOKEN item requires full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ItemBuiltin = BuiltinValidationNCName
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
		{
			name: "item identity requires full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ItemIdentity = SimpleIdentityID
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
		{
			name: "item facets require full path",
			shape: func() SimpleRawListFastPathShape {
				shape := valid
				shape.ItemFacets = FacetPattern
				return shape
			}(),
			want: SimpleRawListFastPathNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleRawListFastPath(tt.shape); got != tt.want {
				t.Fatalf("SimpleRawListFastPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleRawUnionFastPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape SimpleRawUnionFastPathShape
		want  SimpleRawUnionFastPathAction
	}{
		{
			name: "plain union can validate members",
			want: SimpleRawUnionFastPathValidateMembers,
		},
		{
			name: "union facets require full path",
			shape: SimpleRawUnionFastPathShape{
				Facets: FacetPattern,
			},
			want: SimpleRawUnionFastPathNone,
		},
		{
			name: "union identity requires full path",
			shape: SimpleRawUnionFastPathShape{
				Identity: SimpleIdentityID,
			},
			want: SimpleRawUnionFastPathNone,
		},
		{
			name: "raw whitespace requires full path",
			shape: SimpleRawUnionFastPathShape{
				HasWhitespace: true,
			},
			want: SimpleRawUnionFastPathNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleRawUnionFastPath(tt.shape); got != tt.want {
				t.Fatalf("SimpleRawUnionFastPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleRawUnionMember(t *testing.T) {
	t.Parallel()

	valid := SimpleRawUnionMemberShape{
		Known:     true,
		Variety:   SimpleVarietyAtomic,
		Primitive: PrimitiveString,
		Builtin:   BuiltinValidationNone,
		Identity:  SimpleIdentityNone,
	}
	tests := []struct {
		name  string
		shape SimpleRawUnionMemberShape
		want  SimpleRawUnionMemberAction
	}{
		{
			name:  "ordinary member uses raw simple path",
			shape: valid,
			want:  SimpleRawUnionMemberTryRaw,
		},
		{
			name: "missing member disables raw union path",
			shape: func() SimpleRawUnionMemberShape {
				shape := valid
				shape.Known = false
				return shape
			}(),
			want: SimpleRawUnionMemberNone,
		},
		{
			name: "identity member disables raw union path",
			shape: func() SimpleRawUnionMemberShape {
				shape := valid
				shape.Identity = SimpleIdentityID
				return shape
			}(),
			want: SimpleRawUnionMemberNone,
		},
		{
			name: "boolean member uses boolean shortcut",
			shape: func() SimpleRawUnionMemberShape {
				shape := valid
				shape.Primitive = PrimitiveBoolean
				return shape
			}(),
			want: SimpleRawUnionMemberTryBoolean,
		},
		{
			name: "constrained boolean member uses raw simple path",
			shape: func() SimpleRawUnionMemberShape {
				shape := valid
				shape.Primitive = PrimitiveBoolean
				shape.Facets = FacetPattern
				return shape
			}(),
			want: SimpleRawUnionMemberTryRaw,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleRawUnionMember(tt.shape); got != tt.want {
				t.Fatalf("SimpleRawUnionMember() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleValuePrimitiveNeeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape PrimitiveValueNeedShape
		want  PrimitiveValueNeed
	}{
		{
			name: "canonical request needs primitive canonical",
			shape: PrimitiveValueNeedShape{
				Primitive: PrimitiveString,
				Needs:     SimpleNeedCanonical,
			},
			want: PrimitiveNeedCanonical,
		},
		{
			name: "identity simple type needs primitive canonical",
			shape: PrimitiveValueNeedShape{
				Primitive: PrimitiveString,
				Identity:  SimpleIdentityID,
			},
			want: PrimitiveNeedCanonical,
		},
		{
			name: "non-decimal identity key request needs primitive canonical",
			shape: PrimitiveValueNeedShape{
				Primitive: PrimitiveString,
				Needs:     SimpleNeedIdentity,
			},
			want: PrimitiveNeedCanonical,
		},
		{
			name: "decimal identity key can use decimal actual",
			shape: PrimitiveValueNeedShape{
				Primitive: PrimitiveDecimal,
				Needs:     SimpleNeedIdentity,
			},
		},
		{
			name: "enumeration facet needs non-decimal primitive canonical",
			shape: PrimitiveValueNeedShape{
				Facets:    FacetEnumeration,
				Primitive: PrimitiveString,
			},
			want: PrimitiveNeedCanonical,
		},
		{
			name: "decimal enumeration does not need parser canonical",
			shape: PrimitiveValueNeedShape{
				Facets:    FacetEnumeration,
				Primitive: PrimitiveDecimal,
			},
		},
		{
			name: "runtime-owned string length facet does not need primitive length",
			shape: PrimitiveValueNeedShape{
				Facets:    FacetMinLength,
				Primitive: PrimitiveString,
			},
		},
		{
			name: "string derived builtin length stays schema-owned",
			shape: PrimitiveValueNeedShape{
				Facets:    FacetMinLength,
				Primitive: PrimitiveString,
				Builtin:   BuiltinValidationName,
			},
			want: PrimitiveNeedLength,
		},
		{
			name: "schema-owned primitive length facet needs primitive length",
			shape: PrimitiveValueNeedShape{
				Facets:    FacetMinLength,
				Primitive: PrimitiveQName,
			},
			want: PrimitiveNeedLength,
		},
		{
			name: "combined runtime-owned length needs",
			shape: PrimitiveValueNeedShape{
				Facets:    FacetEnumeration | FacetMaxLength,
				Primitive: PrimitiveString,
				Needs:     SimpleNeedIdentity,
			},
			want: PrimitiveNeedCanonical,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValuePrimitiveNeeds(tt.shape); got != tt.want {
				t.Fatalf("SimpleValuePrimitiveNeeds() = %08b, want %08b", got, tt.want)
			}
		})
	}
}

func TestSimpleValueAtomicLengthFacets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape AtomicLengthFacetShape
		want  bool
	}{
		{name: "no length facets", shape: AtomicLengthFacetShape{Primitive: PrimitiveString}},
		{name: "string length", shape: AtomicLengthFacetShape{Facets: FacetLength, Primitive: PrimitiveString}, want: true},
		{name: "anyURI minLength", shape: AtomicLengthFacetShape{Facets: FacetMinLength, Primitive: PrimitiveAnyURI}, want: true},
		{name: "hexBinary maxLength", shape: AtomicLengthFacetShape{Facets: FacetMaxLength, Primitive: PrimitiveHexBinary}, want: true},
		{name: "base64Binary length", shape: AtomicLengthFacetShape{Facets: FacetLength, Primitive: PrimitiveBase64Binary}, want: true},
		{name: "string derived builtin remains schema-owned", shape: AtomicLengthFacetShape{Facets: FacetLength, Primitive: PrimitiveString, Builtin: BuiltinValidationName}},
		{name: "QName remains schema-owned", shape: AtomicLengthFacetShape{Facets: FacetLength, Primitive: PrimitiveQName}},
		{name: "decimal does not own invalid facet shape", shape: AtomicLengthFacetShape{Facets: FacetLength, Primitive: PrimitiveDecimal}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueAtomicLengthFacets(tt.shape); got != tt.want {
				t.Fatalf("SimpleValueAtomicLengthFacets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleValueBuiltinDerivedRuntimeOwned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind BuiltinValidationKind
		want bool
	}{
		{name: "none", kind: BuiltinValidationNone, want: true},
		{name: "integer", kind: BuiltinValidationInteger, want: true},
		{name: "Name", kind: BuiltinValidationName, want: true},
		{name: "NCName", kind: BuiltinValidationNCName, want: true},
		{name: "NMTOKEN", kind: BuiltinValidationNMTOKEN, want: true},
		{name: "language", kind: BuiltinValidationLanguage, want: true},
		{name: "ENTITY", kind: BuiltinValidationEntity, want: true},
		{name: "xml lang", kind: BuiltinValidationXMLLang, want: true},
		{name: "xml space", kind: BuiltinValidationXMLSpace, want: true},
		{name: "invalid", kind: BuiltinValidationKind(99)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueBuiltinDerivedRuntimeOwned(tt.kind); got != tt.want {
				t.Fatalf("SimpleValueBuiltinDerivedRuntimeOwned() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleValueListNeeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape ListSimpleValueNeedShape
		want  ListSimpleValueNeedPlan
	}{
		{
			name: "no projections",
			shape: ListSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
			},
		},
		{
			name: "canonical request needs strings and item canonical",
			shape: ListSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Needs:    SimpleNeedCanonical,
			},
			want: ListSimpleValueNeedPlan{NeedStrings: true, ItemNeeds: SimpleNeedCanonical},
		},
		{
			name: "identity request needs strings and item canonical",
			shape: ListSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Needs:    SimpleNeedIdentity,
			},
			want: ListSimpleValueNeedPlan{NeedStrings: true, ItemNeeds: SimpleNeedCanonical},
		},
		{
			name: "list identity needs strings and item canonical",
			shape: ListSimpleValueNeedShape{
				Identity: SimpleIdentityIDREFList,
			},
			want: ListSimpleValueNeedPlan{NeedStrings: true, ItemNeeds: SimpleNeedCanonical},
		},
		{
			name: "pattern facet needs strings and item canonical",
			shape: ListSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Facets:   FacetPattern,
			},
			want: ListSimpleValueNeedPlan{NeedStrings: true, ItemNeeds: SimpleNeedCanonical},
		},
		{
			name: "enumeration facet needs strings and item canonical",
			shape: ListSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Facets:   FacetEnumeration,
			},
			want: ListSimpleValueNeedPlan{NeedStrings: true, ItemNeeds: SimpleNeedCanonical},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueListNeeds(tt.shape); got != tt.want {
				t.Fatalf("SimpleValueListNeeds() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSimpleValueListFacetPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		facets FacetMask
		want   ListSimpleValueFacetPlan
	}{
		{name: "none"},
		{
			name:   "length",
			facets: FacetLength,
			want:   ListSimpleValueFacetPlan{ValidateLength: true},
		},
		{
			name:   "min length",
			facets: FacetMinLength,
			want:   ListSimpleValueFacetPlan{ValidateLength: true},
		},
		{
			name:   "pattern",
			facets: FacetPattern,
			want:   ListSimpleValueFacetPlan{ValidateLexical: true},
		},
		{
			name:   "enumeration",
			facets: FacetEnumeration,
			want:   ListSimpleValueFacetPlan{ValidateLexical: true},
		},
		{
			name:   "both",
			facets: FacetMaxLength | FacetEnumeration,
			want:   ListSimpleValueFacetPlan{ValidateLength: true, ValidateLexical: true},
		},
		{
			name: "unrelated atomic facet",
			// TotalDigits is valid only for atomic simple values and must not
			// cross the composite list facet callback boundary.
			facets: FacetTotalDigits,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueListFacetPlan(tt.facets); got != tt.want {
				t.Fatalf("SimpleValueListFacetPlan() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSimpleValueUnionMemberNeeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shape UnionSimpleValueNeedShape
		want  SimpleValueNeed
	}{
		{
			name: "no projections",
			shape: UnionSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
			},
		},
		{
			name: "canonical request preserved",
			shape: UnionSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Needs:    SimpleNeedCanonical,
			},
			want: SimpleNeedCanonical,
		},
		{
			name: "identity request adds canonical",
			shape: UnionSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Needs:    SimpleNeedIdentity,
			},
			want: SimpleNeedCanonical | SimpleNeedIdentity,
		},
		{
			name: "union identity adds canonical",
			shape: UnionSimpleValueNeedShape{
				Identity: SimpleIdentityID,
			},
			want: SimpleNeedCanonical,
		},
		{
			name: "enumeration facet adds canonical",
			shape: UnionSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Facets:   FacetEnumeration,
			},
			want: SimpleNeedCanonical,
		},
		{
			name: "pattern facet does not add canonical",
			shape: UnionSimpleValueNeedShape{
				Identity: SimpleIdentityNone,
				Facets:   FacetPattern,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueUnionMemberNeeds(tt.shape); got != tt.want {
				t.Fatalf("SimpleValueUnionMemberNeeds() = %08b, want %08b", got, tt.want)
			}
		})
	}
}

func TestSimpleValueUnionFacetValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		facets FacetMask
		want   bool
	}{
		{name: "none"},
		{name: "pattern", facets: FacetPattern, want: true},
		{name: "enumeration", facets: FacetEnumeration, want: true},
		{name: "length", facets: FacetLength},
		{name: "unrelated atomic facet", facets: FacetTotalDigits},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleValueUnionFacetValidation(tt.facets); got != tt.want {
				t.Fatalf("SimpleValueUnionFacetValidation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtomicSimpleValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		proj AtomicSimpleValueProjection
		want SimpleValue
	}{
		{
			name: "ID payload",
			proj: AtomicSimpleValueProjection{
				Canonical: "abc",
				Type:      1,
				Primitive: PrimitiveString,
				Identity:  SimpleIdentityID,
				Needs:     SimpleNeedIdentity,
			},
			want: SimpleValue{
				Canonical: "abc",
				IDs:       "abc",
				Identity:  SimpleIdentityKey(PrimitiveString, "abc"),
				Type:      1,
			},
		},
		{
			name: "IDREF payload",
			proj: AtomicSimpleValueProjection{
				Canonical: "ref",
				Type:      2,
				Primitive: PrimitiveString,
				Identity:  SimpleIdentityIDREF,
			},
			want: SimpleValue{
				Canonical: "ref",
				IDRefs:    "ref",
				Type:      2,
			},
		},
		{
			name: "decimal identity uses value canonical",
			proj: AtomicSimpleValueProjection{
				Canonical:         "5",
				IdentityCanonical: "5.0",
				Type:              3,
				Primitive:         PrimitiveDecimal,
				Identity:          SimpleIdentityNone,
				Needs:             SimpleNeedIdentity,
			},
			want: SimpleValue{
				Canonical: "5",
				Identity:  SimpleIdentityKey(PrimitiveDecimal, "5.0"),
				Type:      3,
			},
		},
		{
			name: "no identity need omits key",
			proj: AtomicSimpleValueProjection{
				Canonical: "text",
				Type:      4,
				Primitive: PrimitiveString,
				Identity:  SimpleIdentityNone,
			},
			want: SimpleValue{
				Canonical: "text",
				Type:      4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := AtomicSimpleValue(tt.proj); got != tt.want {
				t.Fatalf("AtomicSimpleValue() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestListSimpleValue(t *testing.T) {
	t.Parallel()

	var refs strings.Builder
	AppendSimpleValueIDRefs(&refs, SimpleValue{})
	AppendSimpleValueIDRefs(&refs, SimpleValue{IDRefs: "a"})
	AppendSimpleValueIDRefs(&refs, SimpleValue{IDRefs: "b c"})
	if got, want := refs.String(), "a b c"; got != want {
		t.Fatalf("AppendSimpleValueIDRefs() = %q, want %q", got, want)
	}

	got := ListSimpleValue(ListSimpleValueProjection{
		Canonical:  "a b",
		ItemIDRefs: refs.String(),
		Type:       5,
		Needs:      SimpleNeedIdentity,
	})
	want := SimpleValue{
		Canonical: "a b",
		IDRefs:    "a b c",
		Identity:  SimpleIdentityKey(PrimitiveString, "a b"),
		Type:      5,
	}
	if got != want {
		t.Fatalf("ListSimpleValue() = %#v, want %#v", got, want)
	}
}

func TestValidateSimpleValuePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		value   SimpleValue
		typ     SimpleValuePayloadType
	}{
		{
			name: "valid ID payload",
			value: SimpleValue{
				Canonical: "abc",
				IDs:       "abc",
				Identity:  SimpleIdentityKey(PrimitiveString, "abc"),
			},
			typ: SimpleValuePayloadType{Primitive: PrimitiveString, Identity: SimpleIdentityID},
		},
		{
			name: "valid IDREF list payload uses string identity",
			value: SimpleValue{
				Canonical: "a b",
				IDRefs:    "a b",
				Identity:  SimpleIdentityKey(PrimitiveString, "a b"),
			},
			typ: SimpleValuePayloadType{Primitive: PrimitiveDecimal, Variety: SimpleVarietyList, Identity: SimpleIdentityIDREFList},
		},
		{
			name: "valid non-ID decimal payload uses value canonical",
			value: SimpleValue{
				Canonical: "5",
				Identity:  SimpleIdentityKey(PrimitiveDecimal, "5.0"),
			},
			typ: SimpleValuePayloadType{Primitive: PrimitiveDecimal, Identity: SimpleIdentityNone},
		},
		{
			name: "ID rejects IDREF payload",
			value: SimpleValue{
				Canonical: "abc",
				IDRefs:    "abc",
				Identity:  SimpleIdentityKey(PrimitiveString, "abc"),
			},
			typ:     SimpleValuePayloadType{Primitive: PrimitiveString, Identity: SimpleIdentityID},
			wantErr: "ID payload does not match canonical value",
		},
		{
			name: "IDREF rejects ID payload",
			value: SimpleValue{
				Canonical: "abc",
				IDs:       "abc",
				Identity:  SimpleIdentityKey(PrimitiveString, "abc"),
			},
			typ:     SimpleValuePayloadType{Primitive: PrimitiveString, Identity: SimpleIdentityIDREF},
			wantErr: "IDREF payload does not match canonical value",
		},
		{
			name: "non-ID rejects ID payload",
			value: SimpleValue{
				Canonical: "abc",
				IDs:       "abc",
				Identity:  SimpleIdentityKey(PrimitiveString, "abc"),
			},
			typ:     SimpleValuePayloadType{Primitive: PrimitiveString, Identity: SimpleIdentityNone},
			wantErr: "stores ID payload for non-ID type",
		},
		{
			name: "invalid identity kind",
			value: SimpleValue{
				Canonical: "abc",
				Identity:  SimpleIdentityKey(PrimitiveString, "abc"),
			},
			typ:     SimpleValuePayloadType{Primitive: PrimitiveString, Identity: SimpleIdentityKind(99)},
			wantErr: "stores invalid simple identity kind",
		},
		{
			name: "identity key mismatch",
			value: SimpleValue{
				Canonical: "abc",
				Identity:  SimpleIdentityKey(PrimitiveBoolean, "abc"),
			},
			typ:     SimpleValuePayloadType{Primitive: PrimitiveString, Identity: SimpleIdentityNone},
			wantErr: "identity payload does not match canonical value",
		},
		{
			name: "decimal canonicalization failure",
			value: SimpleValue{
				Canonical: "bad",
				Identity:  SimpleIdentityKey(PrimitiveDecimal, "bad"),
			},
			typ:     SimpleValuePayloadType{Primitive: PrimitiveDecimal, Identity: SimpleIdentityNone},
			wantErr: "identity payload does not match canonical value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleValuePayload(tt.value, tt.typ)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleValuePayload() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleValuePayload() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestBooleanCanonical(t *testing.T) {
	t.Parallel()

	if BooleanCanonical(true) != "true" || BooleanCanonical(false) != "false" {
		t.Fatal("BooleanCanonical() returned non-canonical value")
	}
}

func TestValidateSimpleTypeGraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		nodes   []SimpleTypeGraphNode
	}{
		{
			name: "valid",
			nodes: []SimpleTypeGraphNode{
				atomicSimpleGraphNode(NoSimpleType),
				atomicSimpleGraphNode(0),
				{Base: 0, ListItem: 1, Variety: SimpleVarietyList},
				{Base: 0, ListItem: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1}},
			},
		},
		{
			name: "invalid reference",
			nodes: []SimpleTypeGraphNode{
				{Base: NoSimpleType, ListItem: SimpleTypeID(99), Variety: SimpleVarietyList},
			},
			wantErr: "simple type graph references invalid type",
		},
		{
			name: "cycle",
			nodes: []SimpleTypeGraphNode{
				atomicSimpleGraphNode(1),
				atomicSimpleGraphNode(0),
			},
			wantErr: "simple type graph contains cycle",
		},
		{
			name: "list item is list through union",
			nodes: []SimpleTypeGraphNode{
				atomicSimpleGraphNode(NoSimpleType),
				{Base: 0, ListItem: 0, Variety: SimpleVarietyList},
				{Base: 0, ListItem: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1}},
				{Base: 0, ListItem: 2, Variety: SimpleVarietyList},
			},
			wantErr: "list simple type uses list item type",
		},
		{
			name: "invalid variety",
			nodes: []SimpleTypeGraphNode{
				{Base: NoSimpleType, ListItem: NoSimpleType, Variety: SimpleVariety(99)},
			},
			wantErr: "simple type graph has invalid variety",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeGraph(tt.nodes)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeGraph() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleTypeGraph() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleTypeGraphHasListVariety(t *testing.T) {
	t.Parallel()

	nodes := []SimpleTypeGraphNode{
		atomicSimpleGraphNode(NoSimpleType),
		{Base: 0, ListItem: 0, Variety: SimpleVarietyList},
		{Base: 0, ListItem: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{0}},
		{Base: 0, ListItem: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1}},
		{Base: 0, ListItem: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{4}},
	}
	tests := []struct {
		name string
		id   SimpleTypeID
		want bool
	}{
		{name: "atomic", id: 0},
		{name: "list", id: 1, want: true},
		{name: "union without list", id: 2},
		{name: "union with list", id: 3, want: true},
		{name: "union cycle without list", id: 4},
		{name: "invalid", id: 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SimpleTypeGraphHasListVariety(nodes, tt.id); got != tt.want {
				t.Fatalf("SimpleTypeGraphHasListVariety(..., %d) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func atomicSimpleGraphNode(base SimpleTypeID) SimpleTypeGraphNode {
	return SimpleTypeGraphNode{
		Base:     base,
		ListItem: NoSimpleType,
		Variety:  SimpleVarietyAtomic,
	}
}
