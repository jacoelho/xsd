package runtime

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/xsderrors"
)

func (rt *Schema) validatePublishedSimpleValue(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if value, handled, err := validateSimpleValueRouteReadFast(rt.runtime.SimpleValueRoutes, rt.runtime.Notations, id, lexical, resolve, needs); handled {
		return value, err
	}
	if id == NoSimpleType {
		return SimpleValue{Canonical: lexical, Type: NoSimpleType}, nil
	}
	route, ok := simpleValueRouteReadByID(rt.runtime.SimpleValueRoutes, id)
	if !ok {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	cold, ok := rt.runtime.SimpleValueCold.read(id)
	if !ok {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	typ := simpleValueTypeForRouteAndCold(route, cold)
	switch route.variety {
	case SimpleVarietyAtomic:
		return rt.validatePublishedAtomicSimpleValue(id, &typ, cold, lexical, resolve, needs)
	case SimpleVarietyList:
		return rt.validatePublishedListSimpleValue(id, &typ, cold, lexical, resolve, needs)
	case SimpleVarietyUnion:
		return rt.validatePublishedUnionSimpleValue(&typ, cold, lexical, resolve, needs)
	default:
		return SimpleValue{}, ErrSimpleValueMetadata
	}
}

func (rt *Schema) validatePublishedAtomicSimpleValue(id SimpleTypeID, typ *SimpleValueType, cold *simpleValueColdRead, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
	switch SimpleValueBypass(simpleValueAtomicBypassShape(typ, needs)) {
	case SimpleValueBypassValidateStringPatterns, SimpleValueBypassValidateStringEnumeration:
		if err := validatePublishedStringFacets(typ, cold, normalized, normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateDecimal:
		dec, err := ParseDecimalValue(normalized)
		if err != nil {
			return SimpleValue{}, err
		}
		if err := ValidateDecimalFacets(typ.DecimalFacets, dec); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassNone:
		if value, ok, err := validateAtomicStringSimpleValueFallback(id, *typ, normalized, needs); ok {
			return value, err
		}
	default:
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	if err := validateRuntimeAtomicBuiltin(*typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	if err := validateRuntimeAtomicLengthFacets(*typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	if cold == nil && typ.Facets != 0 {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	var facets SimpleValueFacets
	if cold != nil {
		facets = simpleValueFacetsForColdRead(cold)
	}
	result, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:         *typ,
		Facets:       facets,
		ResolveQName: resolve,
		Notation: func(ns, local string) bool {
			return rt.runtime.Notations[ExpandedName{Namespace: ns, Local: local}]
		},
		Normalized: normalized,
		Needs: SimpleValuePrimitiveNeeds(PrimitiveValueNeedShape{
			Facets:    typ.Facets,
			Primitive: typ.Primitive,
			Builtin:   typ.Builtin,
			Identity:  typ.Identity,
			Needs:     needs,
		}),
		Present: true,
	})
	if err != nil {
		return SimpleValue{}, err
	}
	return AtomicSimpleValue(AtomicSimpleValueProjection{
		Canonical:         result.Canonical,
		IdentityCanonical: result.IdentityCanonical,
		Type:              id,
		Primitive:         typ.Primitive,
		Identity:          typ.Identity,
		Needs:             needs,
	}), nil
}

func (rt *Schema) validatePublishedListSimpleValue(id SimpleTypeID, typ *SimpleValueType, cold *simpleValueColdRead, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	needPlan := SimpleValueListNeeds(ListSimpleValueNeedShape{Facets: typ.Facets, Identity: typ.Identity, Needs: needs})
	var canonical, normalized, refs strings.Builder
	var validateErr error
	count := uint32(0)
	forEachSimpleValueListItem(lexical, func(item string) bool {
		itemValue, err := rt.validatePublishedSimpleValue(typ.ListItem, item, resolve, needPlan.ItemNeeds)
		if err != nil {
			validateErr = err
			return false
		}
		if needPlan.NeedStrings {
			if count > 0 {
				canonical.WriteByte(' ')
				normalized.WriteByte(' ')
			}
			canonical.WriteString(itemValue.Canonical)
			normalized.WriteString(item)
		}
		AppendSimpleValueIDRefs(&refs, itemValue)
		count++
		return true
	})
	if validateErr != nil {
		return SimpleValue{}, validateErr
	}
	canonicalText, normalizedText := "", ""
	if needPlan.NeedStrings {
		canonicalText, normalizedText = canonical.String(), normalized.String()
	}
	plan := SimpleValueListFacetPlan(typ.Facets)
	if plan.ValidateLength {
		if err := validateSimpleValueLengthFacets(*typ, count); err != nil {
			return SimpleValue{}, err
		}
	}
	if plan.ValidateLexical {
		if err := validatePublishedStringFacets(typ, cold, normalizedText, canonicalText); err != nil {
			return SimpleValue{}, err
		}
	}
	return ListSimpleValue(ListSimpleValueProjection{Canonical: canonicalText, ItemIDRefs: refs.String(), Type: id, Needs: needs}), nil
}

func (rt *Schema) validatePublishedUnionSimpleValue(typ *SimpleValueType, cold *simpleValueColdRead, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if cold == nil || len(typ.UnionMembers) == 0 {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
	memberNeeds := SimpleValueUnionMemberNeeds(UnionSimpleValueNeedShape{Facets: typ.Facets, Identity: typ.Identity, Needs: needs})
	var unsupportedErr error
	for _, member := range typ.UnionMembers {
		value, err := rt.validatePublishedSimpleValue(member, normalized, resolve, memberNeeds)
		if err == nil {
			if SimpleValueUnionFacetValidation(typ.Facets) {
				if facetErr := validatePublishedStringFacets(typ, cold, normalized, value.Canonical); facetErr != nil {
					return SimpleValue{}, facetErr
				}
			}
			return value, nil
		}
		if unsupportedErr == nil && xsderrors.IsUnsupported(err) {
			unsupportedErr = err
		}
	}
	if unsupportedErr != nil {
		return SimpleValue{}, unsupportedErr
	}
	return SimpleValue{}, errors.New("value does not match any union member")
}

func validatePublishedStringFacets(typ *SimpleValueType, cold *simpleValueColdRead, normalized, canonical string) error {
	if cold == nil {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetPattern != 0 && len(typ.StringFacets.Patterns) == 0 {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetEnumeration != 0 && len(cold.enumeration) == 0 {
		return ErrSimpleValueMetadata
	}
	if err := ValidateStringPatterns(typ.StringFacets.Patterns, normalized); err != nil {
		return err
	}
	if typ.Facets&FacetEnumeration == 0 {
		return nil
	}
	for _, literal := range cold.enumeration {
		if literal.Canonical == canonical {
			return nil
		}
	}
	return errors.New("enumeration facet failed")
}

func (rt *Schema) validatePublishedRawSimpleValue(id SimpleTypeID, raw []byte) (bool, error) {
	route, ok := simpleValueRouteReadByID(rt.runtime.SimpleValueRoutes, id)
	if !ok {
		return false, ErrSimpleValueMetadata
	}
	cold, ok := rt.runtime.SimpleValueCold.read(id)
	if !ok {
		return false, ErrSimpleValueMetadata
	}
	switch route.variety {
	case SimpleVarietyAtomic:
		return validatePublishedRawAtomic(route, cold, raw)
	case SimpleVarietyList:
		return rt.validatePublishedRawList(route, raw)
	case SimpleVarietyUnion:
		return rt.validatePublishedRawUnion(route, cold, raw)
	default:
		return false, ErrSimpleValueMetadata
	}
}

func validatePublishedRawAtomic(route *simpleValueRouteRead, cold *simpleValueColdRead, raw []byte) (bool, error) {
	if route.primitive == PrimitiveString && route.builtin == BuiltinValidationNone &&
		route.identity == SimpleIdentityNone && route.facets != 0 && route.facets&^runtimeLengthFacetMask == 0 {
		if cold == nil {
			return false, ErrSimpleValueMetadata
		}
		return true, validateRawStringLength(raw, route.whitespace, lengthFacetValues(cold.facets))
	}
	if route.rawBypass != SimpleValueBypassValidateStringPatterns && route.rawBypass != SimpleValueBypassValidateStringEnumeration {
		return validateRawAtomicSimpleValueRoute(route, raw)
	}
	if cold == nil {
		return false, ErrSimpleValueMetadata
	}
	rawNorm, normalized := rawEqualsNormalizedString(route.whitespace, raw)
	if !normalized {
		return false, nil
	}
	if route.rawBypass == SimpleValueBypassValidateStringPatterns {
		return true, ValidateRawStringPatterns(cold.facets.Patterns, rawNorm)
	}
	for _, literal := range cold.enumeration {
		if byteStringEqual(literal.Canonical, rawNorm) {
			return true, nil
		}
	}
	return true, errors.New("enumeration facet failed")
}

func (rt *Schema) validatePublishedRawList(route *simpleValueRouteRead, raw []byte) (bool, error) {
	if route.identity != SimpleIdentityNone || route.facets != 0 {
		return false, nil
	}
	item, ok := simpleValueRouteReadByID(rt.runtime.SimpleValueRoutes, route.listItem)
	if !ok {
		return false, ErrSimpleValueMetadata
	}
	if item.variety == SimpleVarietyAtomic && item.builtin == BuiltinValidationNMTOKEN && item.identity == SimpleIdentityNone && item.facets == 0 {
		return true, ValidateNMTOKENListBytes(raw)
	}
	return false, nil
}

func (rt *Schema) validatePublishedRawUnion(route *simpleValueRouteRead, cold *simpleValueColdRead, raw []byte) (bool, error) {
	if cold == nil || len(cold.union) == 0 {
		return false, ErrSimpleValueMetadata
	}
	if route.identity != SimpleIdentityNone || route.facets != 0 || lex.HasXMLWhitespaceBytes(raw) {
		return false, nil
	}
	for _, member := range cold.union {
		handled, err := rt.validatePublishedRawSimpleValue(member, raw)
		if !handled {
			return false, nil
		}
		if err == nil {
			return true, nil
		}
	}
	return true, errors.New("value does not match any union member")
}

func validateRawStringLength(raw []byte, whitespace WhitespaceMode, facets LengthFacetValues) error {
	var count uint64
	seen, pendingSpace := false, false
	for len(raw) != 0 {
		r, size := utf8.DecodeRune(raw)
		if r == utf8.RuneError && size == 1 {
			return errors.New("invalid UTF-8 string")
		}
		raw = raw[size:]
		if whitespace == WhitespaceCollapse && isXMLWhitespaceRune(r) {
			if seen {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			count++
			pendingSpace = false
		}
		count++
		seen = true
		if count > math.MaxUint32 {
			return fmt.Errorf("string length exceeds %d", uint64(math.MaxUint32))
		}
	}
	return ValidateLengthFacets(facets, uint32(count))
}

func isXMLWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
