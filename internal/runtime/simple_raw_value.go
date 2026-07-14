package runtime

import (
	"errors"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/uriref"
)

type rawSimpleValueResolver struct {
	runtime *schemaRuntime
	scratch *StringPatternScratch
}

type rawSimpleValueView struct {
	route *simpleValueRouteRead
	cold  *simpleValueColdRead
}

func (r rawSimpleValueResolver) resolveRawSimpleValue(id SimpleTypeID) (rawSimpleValueView, bool) {
	if r.runtime == nil {
		return rawSimpleValueView{}, false
	}
	route, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id)
	if !ok {
		return rawSimpleValueView{}, false
	}
	cold, ok := r.runtime.SimpleTypeCold.read(id)
	if !ok || cold == nil && (route.facets != 0 || route.variety == SimpleVarietyUnion) {
		return rawSimpleValueView{}, false
	}
	return rawSimpleValueView{route: route, cold: cold}, true
}

func validateRawSimpleValuePatterns(cold *simpleValueColdRead, raw []byte, scratch *StringPatternScratch) error {
	if cold == nil {
		return ErrSimpleValueMetadata
	}
	return validateRawStringPatternStepReadsWithScratch(cold.facets.patterns, raw, scratch)
}

func rawLengthFacets(cold *simpleValueColdRead) LengthFacetValues {
	if cold == nil {
		return LengthFacetValues{}
	}
	return cold.facets.lengthValues()
}

func (v rawSimpleValueView) rawUnionMemberCount() (int, bool) {
	if v.cold == nil || len(v.cold.union) == 0 {
		return 0, false
	}
	return len(v.cold.union), true
}

func (v rawSimpleValueView) rawUnionMember(index int) (SimpleTypeID, bool) {
	if v.cold == nil || index < 0 || index >= len(v.cold.union) {
		return NoSimpleType, false
	}
	return v.cold.union[index], true
}

func rawStringEnumeration(cold *simpleValueColdRead, normalized []byte) (bool, bool) {
	if cold == nil {
		return false, false
	}
	for _, literal := range cold.enumeration {
		if byteStringEqual(literal.canonical, normalized) {
			return true, true
		}
	}
	return false, true
}

func validateResolvedRawSimpleValue(resolver rawSimpleValueResolver, id SimpleTypeID, raw []byte) (bool, error) {
	typ, ok := resolver.resolveRawSimpleValue(id)
	if !ok {
		if id != NoSimpleType {
			return false, ErrSimpleValueMetadata
		}
		return false, nil
	}
	return validateRawSimpleValueView(resolver, id, typ, raw)
}

func validateRawSimpleValueView(resolver rawSimpleValueResolver, id SimpleTypeID, typ rawSimpleValueView, raw []byte) (bool, error) {
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: typ.route.variety, Known: true}) {
	case SimpleValueRouteAtomic:
		return validateRawAtomicSimpleValue(typ.route, typ.cold, raw, resolver.scratch)
	case SimpleValueRouteList:
		return validateRawListSimpleValue(resolver, typ, raw)
	case SimpleValueRouteUnion:
		return validateRawUnionSimpleValue(resolver, typ, raw)
	case SimpleValueRouteInvalid:
		return false, ErrSimpleValueMetadata
	case SimpleValueRouteUntyped, SimpleValueRouteMissing:
		return false, nil
	}
	return false, nil
}

func validateRawAtomicSimpleValue(
	route *simpleValueRouteRead,
	cold *simpleValueColdRead,
	raw []byte,
	scratch *StringPatternScratch,
) (bool, error) {
	facets := route.facets
	primitive := route.primitive
	if primitive == PrimitiveString && route.builtin == BuiltinValidationNone &&
		route.identity == SimpleIdentityNone && facets != 0 && facets&^runtimeLengthFacetMask == 0 {
		return true, validateRawStringLength(raw, route.whitespace, rawLengthFacets(cold))
	}
	switch route.rawBypass {
	case SimpleValueBypassAcceptString:
		return true, nil
	case SimpleValueBypassValidateStringPatterns:
		rawNorm, ok := rawEqualsNormalizedString(route.whitespace, raw)
		if !ok {
			return false, nil
		}
		return true, validateRawSimpleValuePatterns(cold, rawNorm, scratch)
	case SimpleValueBypassValidateStringEnumeration:
		rawNorm, ok := rawEqualsNormalizedString(route.whitespace, raw)
		if !ok {
			return false, nil
		}
		matched, known := rawStringEnumeration(cold, rawNorm)
		if !known {
			return false, ErrSimpleValueMetadata
		}
		if !matched {
			return true, errors.New("enumeration facet failed")
		}
		return true, nil
	case SimpleValueBypassValidateInt:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return true, ValidateFastIntLexical(raw)
	case SimpleValueBypassValidateDecimal:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		return ValidateFastDecimalLexical(RawDecimalFastPathShape{
			Facets:       facets,
			MinInclusive: route.minInclusive,
			MaxInclusive: route.maxInclusive,
		}, raw)
	case SimpleValueBypassValidateAnyURI:
		if lex.HasXMLWhitespaceBytes(raw) {
			return false, nil
		}
		_, err := uriref.Check(raw)
		return true, err
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
		return true, ValidateFloatLexical(raw, simpleValueFloatBits(primitive))
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
		return true, ValidateTemporalLexical(primitive, raw)
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

func validateRawListSimpleValue(resolver rawSimpleValueResolver, typ rawSimpleValueView, raw []byte) (bool, error) {
	shape, ok := rawSimpleListFastPathShape(resolver, typ)
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

func validateRawUnionSimpleValue(resolver rawSimpleValueResolver, typ rawSimpleValueView, raw []byte) (bool, error) {
	switch SimpleRawUnionFastPath(SimpleRawUnionFastPathShape{
		Facets:        typ.route.facets,
		Identity:      typ.route.identity,
		HasWhitespace: lex.HasXMLWhitespaceBytes(raw),
	}) {
	case SimpleRawUnionFastPathValidateMembers:
	case SimpleRawUnionFastPathNone:
		return false, nil
	}

	memberCount, ok := typ.rawUnionMemberCount()
	if !ok || memberCount < 0 {
		return false, ErrSimpleValueMetadata
	}
	for i := range memberCount {
		member, ok := typ.rawUnionMember(i)
		if !ok {
			return false, ErrSimpleValueMetadata
		}
		memberType, ok := resolver.resolveRawSimpleValue(member)
		if !ok {
			return false, ErrSimpleValueMetadata
		}
		switch SimpleRawUnionMember(rawSimpleUnionMemberShape(memberType, ok)) {
		case SimpleRawUnionMemberTryBoolean:
			if BooleanLexicalOK(raw) {
				return true, nil
			}
		case SimpleRawUnionMemberTryRaw:
			ok, err := validateRawSimpleValueView(resolver, member, memberType, raw)
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

func rawSimpleListFastPathShape(resolver rawSimpleValueResolver, typ rawSimpleValueView) (SimpleRawListFastPathShape, bool) {
	shape := SimpleRawListFastPathShape{
		ListFacets:   typ.route.facets,
		ListIdentity: typ.route.identity,
	}
	item, ok := resolver.resolveRawSimpleValue(typ.route.listItem)
	if !ok {
		return shape, false
	}
	shape.ItemKnown = true
	shape.ItemFacets = item.route.facets
	shape.ItemVariety = item.route.variety
	shape.ItemBuiltin = item.route.builtin
	shape.ItemIdentity = item.route.identity
	return shape, true
}

func rawSimpleUnionMemberShape(typ rawSimpleValueView, ok bool) SimpleRawUnionMemberShape {
	if !ok {
		return SimpleRawUnionMemberShape{}
	}
	return SimpleRawUnionMemberShape{
		Facets:    typ.route.facets,
		Variety:   typ.route.variety,
		Primitive: typ.route.primitive,
		Builtin:   typ.route.builtin,
		Identity:  typ.route.identity,
		Fast:      typ.route.fast,
		Known:     true,
	}
}
