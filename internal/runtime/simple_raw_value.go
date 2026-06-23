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
	ListItem            SimpleTypeID
	Facets              FacetMask
	Variety             SimpleVariety
	Primitive           PrimitiveKind
	Builtin             BuiltinValidationKind
	Whitespace          WhitespaceMode
	Identity            SimpleIdentityKind
	Fast                SimpleFastKind
}

// RawSimpleValueCallbacks supplies schema-private facts used by the
// runtime-owned raw simple-value dispatcher.
type RawSimpleValueCallbacks struct {
	Type                     func(id SimpleTypeID) (RawSimpleValueType, bool)
	ForEachUnionMember       func(id SimpleTypeID, yield func(SimpleTypeID) bool)
	ForEachStringEnumeration func(id SimpleTypeID, yield func(string) bool)
}

// ValidateRawSimpleValue validates raw text through a runtime-owned executor or
// remaining schema-private executor when the frozen simple-type shape admits a
// raw fast path.
func ValidateRawSimpleValue(cb RawSimpleValueCallbacks, id SimpleTypeID, raw []byte) (bool, error) {
	typ, ok := cb.Type(id)
	if !ok {
		if id != NoSimpleType {
			return false, ErrSimpleValueMetadata
		}
		return false, nil
	}
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: typ.Variety, Known: true}) {
	case SimpleValueRouteAtomic:
		return validateRawAtomicSimpleValue(cb, id, typ, raw)
	case SimpleValueRouteList:
		return validateRawListSimpleValue(cb, typ, raw)
	case SimpleValueRouteUnion:
		return validateRawUnionSimpleValue(cb, id, typ, raw)
	case SimpleValueRouteInvalid:
		return false, ErrSimpleValueMetadata
	case SimpleValueRouteUntyped, SimpleValueRouteMissing:
		return false, nil
	}
	return false, nil
}

// ValidateRawSimpleValueFromTypeReads validates raw text directly from the
// published simple-value type reads, avoiding per-value raw projection copies.
func ValidateRawSimpleValueFromTypeReads(types []SimpleValueTypeRead, facets SimpleValueFacetReadTable, id SimpleTypeID, raw []byte) (bool, error) {
	read, ok := simpleValueTypeReadByID(types, id)
	if !ok {
		if id != NoSimpleType {
			return false, ErrSimpleValueMetadata
		}
		return false, nil
	}
	return validateRawSimpleValueType(types, facets, id, &read.Type, raw)
}

func validateRawSimpleValueType(types []SimpleValueTypeRead, facets SimpleValueFacetReadTable, id SimpleTypeID, typ *SimpleValueType, raw []byte) (bool, error) {
	switch typ.Variety {
	case SimpleVarietyAtomic:
		return validateRawAtomicSimpleValueType(facets, id, typ, raw)
	case SimpleVarietyList:
		return validateRawListSimpleValueType(types, typ, raw)
	case SimpleVarietyUnion:
		return validateRawUnionSimpleValueType(types, facets, typ, raw)
	}
	return false, ErrSimpleValueMetadata
}

func validateRawAtomicSimpleValueType(facets SimpleValueFacetReadTable, id SimpleTypeID, typ *SimpleValueType, raw []byte) (bool, error) {
	action := typ.RawBypass
	if action == SimpleValueBypassNone {
		action = SimpleValueBypass(simpleValueAtomicBypassShape(typ, 0))
	}
	switch action {
	case SimpleValueBypassAcceptString:
		return true, nil
	case SimpleValueBypassValidateStringPatterns:
		rawNorm, ok := rawEqualsNormalizedString(typ.Whitespace, raw)
		if !ok {
			return false, nil
		}
		return true, ValidateRawStringPatterns(typ.StringFacets.Patterns, rawNorm)
	case SimpleValueBypassValidateStringEnumeration:
		rawNorm, ok := rawEqualsNormalizedString(typ.Whitespace, raw)
		if !ok {
			return false, nil
		}
		if len(typ.StringFacets.CanonicalEnumeration) != 0 {
			return true, validateRawStringEnumeration(typ.StringFacets.CanonicalEnumeration, rawNorm)
		}
		return true, validateRawStringEnumerationFromTable(facets, id, rawNorm)
	case SimpleValueBypassValidateInt:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateFastIntLexical(raw)
	case SimpleValueBypassValidateDecimal:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return validateFastDecimalLexicalPublished(RawDecimalFastPathShape{
			Facets:       typ.Facets,
			MinInclusive: typ.DecimalMinInclusive,
			MaxInclusive: typ.DecimalMaxInclusive,
		}, raw)
	case SimpleValueBypassValidateAnyURI:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateAnyURILexical(raw)
	case SimpleValueBypassValidateHexBinary:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateHexBinaryLexical(raw)
	case SimpleValueBypassValidateBase64Binary:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateBase64BinaryLexical(raw)
	case SimpleValueBypassValidateFloat:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateFloatLexical(raw, simpleValueFloatBits(typ.Primitive))
	case SimpleValueBypassValidateDuration:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateDurationLexical(raw)
	case SimpleValueBypassValidateBoolean:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateBooleanLexical(raw)
	case SimpleValueBypassValidateTemporal:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateTemporalLexical(typ.Primitive, raw)
	case SimpleValueBypassValidateDate:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return ValidateFastDateLexical(raw)
	case SimpleValueBypassNone:
		return false, nil
	}
	return false, nil
}

func validateRawStringEnumeration(enumeration []string, rawNorm []byte) error {
	for _, lit := range enumeration {
		if byteStringEqual(lit, rawNorm) {
			return nil
		}
	}
	return errors.New("enumeration facet failed")
}

func validateRawStringEnumerationFromTable(facets SimpleValueFacetReadTable, id SimpleTypeID, rawNorm []byte) error {
	f, ok := facets.Facets(id)
	if !ok {
		return ErrSimpleValueMetadata
	}
	for _, lit := range f.Enumeration {
		if byteStringEqual(lit.Canonical, rawNorm) {
			return nil
		}
	}
	return errors.New("enumeration facet failed")
}

func validateRawListSimpleValueType(types []SimpleValueTypeRead, typ *SimpleValueType, raw []byte) (bool, error) {
	if typ.Identity != SimpleIdentityNone || typ.Facets != 0 {
		return false, nil
	}
	item, ok := simpleValueTypeReadByID(types, typ.ListItem)
	if !ok {
		return false, ErrSimpleValueMetadata
	}
	if item.Type.Variety == SimpleVarietyAtomic &&
		item.Type.Builtin == BuiltinValidationNMTOKEN &&
		item.Type.Identity == SimpleIdentityNone &&
		item.Type.Facets == 0 {
		return true, ValidateNMTOKENListBytes(raw)
	}
	return false, nil
}

func validateRawUnionSimpleValueType(types []SimpleValueTypeRead, facets SimpleValueFacetReadTable, typ *SimpleValueType, raw []byte) (bool, error) {
	if typ.Identity != SimpleIdentityNone || typ.Facets != 0 || lex.HasXMLWhitespaceBytes(raw) {
		return false, nil
	}

	var matched, unhandled bool
	for _, member := range typ.UnionMembers {
		memberType, ok := simpleValueTypeReadByID(types, member)
		if !ok {
			return false, ErrSimpleValueMetadata
		}
		if memberType.Type.Variety != SimpleVarietyAtomic || memberType.Type.Identity != SimpleIdentityNone {
			unhandled = true
			break
		}
		if memberType.Type.Primitive == PrimitiveBoolean &&
			memberType.Type.Builtin == BuiltinValidationNone &&
			memberType.Type.Facets == 0 {
			if BooleanLexicalOK(raw) {
				matched = true
				break
			}
			continue
		}
		ok, err := validateRawSimpleValueType(types, facets, member, &memberType.Type, raw)
		if !ok {
			unhandled = true
			break
		}
		if err == nil {
			matched = true
			break
		}
	}
	if matched {
		return true, nil
	}
	if unhandled {
		return false, nil
	}
	return true, errors.New("value does not match any union member")
}

func validateRawAtomicSimpleValue(cb RawSimpleValueCallbacks, id SimpleTypeID, typ RawSimpleValueType, raw []byte) (bool, error) {
	action := SimpleValueBypass(rawSimpleValueBypassShape(typ))
	shape, rawNorm := rawAtomicFastPathFacts(action, typ.Whitespace, raw)
	switch SimpleRawAtomicFastPath(shape) {
	case SimpleRawAtomicFastPathAccept:
		return true, nil
	case SimpleRawAtomicFastPathValidateStringPatterns:
		return true, ValidateRawStringPatterns(typ.StringPatterns, rawNorm)
	case SimpleRawAtomicFastPathValidateStringEnumeration:
		return true, ValidateRawStringEnumeration(id, cb.ForEachStringEnumeration, rawNorm)
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

func validateRawListSimpleValue(cb RawSimpleValueCallbacks, typ RawSimpleValueType, raw []byte) (bool, error) {
	shape, ok := rawSimpleListFastPathShape(cb, typ)
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

func validateRawUnionSimpleValue(cb RawSimpleValueCallbacks, id SimpleTypeID, typ RawSimpleValueType, raw []byte) (bool, error) {
	switch SimpleRawUnionFastPath(SimpleRawUnionFastPathShape{
		Facets:        typ.Facets,
		Identity:      typ.Identity,
		HasWhitespace: lex.HasXMLWhitespaceBytes(raw),
	}) {
	case SimpleRawUnionFastPathValidateMembers:
	case SimpleRawUnionFastPathNone:
		return false, nil
	}

	var matched, unhandled bool
	var metadataErr error
	cb.ForEachUnionMember(id, func(member SimpleTypeID) bool {
		memberType, ok := cb.Type(member)
		if !ok {
			metadataErr = ErrSimpleValueMetadata
			return false
		}
		switch SimpleRawUnionMember(rawSimpleUnionMemberShape(memberType, ok)) {
		case SimpleRawUnionMemberTryBoolean:
			if BooleanLexicalOK(raw) {
				matched = true
				return false
			}
			return true
		case SimpleRawUnionMemberTryRaw:
			ok, err := ValidateRawSimpleValue(cb, member, raw)
			if !ok {
				unhandled = true
				return false
			}
			if err == nil {
				matched = true
				return false
			}
			return true
		case SimpleRawUnionMemberNone:
			unhandled = true
			return false
		}
		return false
	})
	if metadataErr != nil {
		return false, metadataErr
	}
	if matched {
		return true, nil
	}
	if unhandled {
		return false, nil
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

func rawSimpleListFastPathShape(cb RawSimpleValueCallbacks, typ RawSimpleValueType) (SimpleRawListFastPathShape, bool) {
	shape := SimpleRawListFastPathShape{
		ListFacets:   typ.Facets,
		ListIdentity: typ.Identity,
	}
	item, ok := cb.Type(typ.ListItem)
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
