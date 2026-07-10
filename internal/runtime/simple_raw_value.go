package runtime

import (
	"errors"

	"github.com/jacoelho/xsd/internal/lex"
)

// RawSimpleValueType is the runtime-owned projection needed to route raw
// simple-value validation.
type RawSimpleValueType struct {
	DecimalMinInclusive RawDecimalBound
	DecimalMaxInclusive RawDecimalBound
	StringPatterns      []StringPatternGroup
	LengthFacets        LengthFacetValues
	ListItem            SimpleTypeID
	Facets              FacetMask
	Variety             SimpleVariety
	Primitive           PrimitiveKind
	Builtin             BuiltinValidationKind
	Whitespace          WhitespaceMode
	Identity            SimpleIdentityKind
	Fast                SimpleFastKind
	RawBypass           SimpleValueBypassAction
}

type rawSimpleValueMetadataReader interface {
	rawSimpleValueType(id SimpleTypeID) (RawSimpleValueType, bool)
	rawSimpleValueUnionMemberCount(id SimpleTypeID) (int, bool)
	rawSimpleValueUnionMember(id SimpleTypeID, index int) (SimpleTypeID, bool)
	rawSimpleValueStringEnumeration(id SimpleTypeID, normalized []byte) (bool, bool)
}

func validateRawSimpleValue[R rawSimpleValueMetadataReader](reader R, id SimpleTypeID, raw []byte) (bool, error) {
	typ, ok := reader.rawSimpleValueType(id)
	if !ok {
		if id != NoSimpleType {
			return false, ErrSimpleValueMetadata
		}
		return false, nil
	}
	return validateRawSimpleValueType(reader, id, typ, raw)
}

func validateRawSimpleValueType[R rawSimpleValueMetadataReader](reader R, id SimpleTypeID, typ RawSimpleValueType, raw []byte) (bool, error) {
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: typ.Variety, Known: true}) {
	case SimpleValueRouteAtomic:
		return validateRawAtomicSimpleValue(reader, id, typ, raw)
	case SimpleValueRouteList:
		return validateRawListSimpleValue(reader, typ, raw)
	case SimpleValueRouteUnion:
		return validateRawUnionSimpleValue(reader, id, typ, raw)
	case SimpleValueRouteInvalid:
		return false, ErrSimpleValueMetadata
	case SimpleValueRouteUntyped, SimpleValueRouteMissing:
		return false, nil
	}
	return false, nil
}

func validateRawAtomicSimpleValue[R rawSimpleValueMetadataReader](reader R, id SimpleTypeID, typ RawSimpleValueType, raw []byte) (bool, error) {
	if typ.Primitive == PrimitiveString && typ.Builtin == BuiltinValidationNone &&
		typ.Identity == SimpleIdentityNone && typ.Facets != 0 && typ.Facets&^runtimeLengthFacetMask == 0 {
		return true, validateRawStringLength(raw, typ.Whitespace, typ.LengthFacets)
	}
	action := typ.RawBypass
	if action == SimpleValueBypassNone {
		action = SimpleValueBypass(rawSimpleValueBypassShape(typ))
	}
	shape, rawNorm := rawAtomicFastPathFacts(action, typ.Whitespace, raw)
	switch SimpleRawAtomicFastPath(shape) {
	case SimpleRawAtomicFastPathAccept:
		return true, nil
	case SimpleRawAtomicFastPathValidateStringPatterns:
		return true, ValidateRawStringPatterns(typ.StringPatterns, rawNorm)
	case SimpleRawAtomicFastPathValidateStringEnumeration:
		matched, known := reader.rawSimpleValueStringEnumeration(id, rawNorm)
		if !known {
			return false, ErrSimpleValueMetadata
		}
		if !matched {
			return true, errors.New("enumeration facet failed")
		}
		return true, nil
	case SimpleRawAtomicFastPathValidateInt:
		return true, ValidateFastIntLexical(raw)
	case SimpleRawAtomicFastPathValidateDecimal:
		return ValidateFastDecimalLexical(RawDecimalFastPathShape{
			Facets:       typ.Facets,
			MinInclusive: typ.DecimalMinInclusive,
			MaxInclusive: typ.DecimalMaxInclusive,
		}, raw)
	case SimpleRawAtomicFastPathValidateAnyURI:
		return true, ValidateAnyURILexical(raw)
	case SimpleRawAtomicFastPathValidateHexBinary:
		return true, ValidateHexBinaryLexical(raw)
	case SimpleRawAtomicFastPathValidateBase64Binary:
		return true, ValidateBase64BinaryLexical(raw)
	case SimpleRawAtomicFastPathValidateFloat:
		return true, ValidateFloatLexical(raw, simpleValueFloatBits(typ.Primitive))
	case SimpleRawAtomicFastPathValidateDuration:
		return true, ValidateDurationLexical(raw)
	case SimpleRawAtomicFastPathValidateBoolean:
		return true, ValidateBooleanLexical(raw)
	case SimpleRawAtomicFastPathValidateTemporal:
		return true, ValidateTemporalLexical(typ.Primitive, raw)
	case SimpleRawAtomicFastPathValidateDate:
		return ValidateFastDateLexical(raw)
	case SimpleRawAtomicFastPathNone:
		return false, nil
	}
	return false, nil
}

func validateRawListSimpleValue[R rawSimpleValueMetadataReader](reader R, typ RawSimpleValueType, raw []byte) (bool, error) {
	shape, ok := rawSimpleListFastPathShape(reader, typ)
	if !ok {
		return false, ErrSimpleValueMetadata
	}
	switch SimpleRawListFastPath(shape) {
	case SimpleRawListFastPathValidateNMTOKENList:
		return true, ValidateNMTOKENListBytes(raw)
	case SimpleRawListFastPathNone:
		return false, nil
	}
	return false, nil
}

func validateRawUnionSimpleValue[R rawSimpleValueMetadataReader](reader R, id SimpleTypeID, typ RawSimpleValueType, raw []byte) (bool, error) {
	switch SimpleRawUnionFastPath(SimpleRawUnionFastPathShape{
		Facets:        typ.Facets,
		Identity:      typ.Identity,
		HasWhitespace: lex.HasXMLWhitespaceBytes(raw),
	}) {
	case SimpleRawUnionFastPathValidateMembers:
	case SimpleRawUnionFastPathNone:
		return false, nil
	}

	memberCount, ok := reader.rawSimpleValueUnionMemberCount(id)
	if !ok || memberCount < 0 {
		return false, ErrSimpleValueMetadata
	}
	for i := range memberCount {
		member, ok := reader.rawSimpleValueUnionMember(id, i)
		if !ok {
			return false, ErrSimpleValueMetadata
		}
		memberType, ok := reader.rawSimpleValueType(member)
		if !ok {
			return false, ErrSimpleValueMetadata
		}
		switch SimpleRawUnionMember(rawSimpleUnionMemberShape(memberType, ok)) {
		case SimpleRawUnionMemberTryBoolean:
			if BooleanLexicalOK(raw) {
				return true, nil
			}
		case SimpleRawUnionMemberTryRaw:
			ok, err := validateRawSimpleValueType(reader, member, memberType, raw)
			if !ok {
				return false, nil
			}
			if err == nil {
				return true, nil
			}
		case SimpleRawUnionMemberNone:
			return false, nil
		}
	}
	return true, errors.New("value does not match any union member")
}

func rawSimpleValueBypassShape(typ RawSimpleValueType) SimpleValueBypassShape {
	return SimpleValueBypassShape{
		Facets:    typ.Facets,
		Variety:   typ.Variety,
		Primitive: typ.Primitive,
		Builtin:   typ.Builtin,
		Identity:  typ.Identity,
		Fast:      typ.Fast,
	}
}

func rawSimpleListFastPathShape[R rawSimpleValueMetadataReader](reader R, typ RawSimpleValueType) (SimpleRawListFastPathShape, bool) {
	shape := SimpleRawListFastPathShape{
		ListFacets:   typ.Facets,
		ListIdentity: typ.Identity,
	}
	item, ok := reader.rawSimpleValueType(typ.ListItem)
	if !ok {
		return shape, false
	}
	shape.ItemKnown = true
	shape.ItemFacets = item.Facets
	shape.ItemVariety = item.Variety
	shape.ItemBuiltin = item.Builtin
	shape.ItemIdentity = item.Identity
	return shape, true
}

func rawSimpleUnionMemberShape(typ RawSimpleValueType, ok bool) SimpleRawUnionMemberShape {
	if !ok {
		return SimpleRawUnionMemberShape{}
	}
	return SimpleRawUnionMemberShape{
		Facets:    typ.Facets,
		Variety:   typ.Variety,
		Primitive: typ.Primitive,
		Builtin:   typ.Builtin,
		Identity:  typ.Identity,
		Fast:      typ.Fast,
		Known:     true,
	}
}
