package runtime

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/jacoelho/xsd/xsderrors"
)

func (rt *Schema) validatePublishedSimpleValue(id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if value, handled, err := validateSimpleValueRouteReadFast(rt.runtime.SimpleValueRoutes, rt.runtime.Notations, id, lexical, resolve, needs); handled {
		return value, err
	}
	return validateSimpleValue(publishedSimpleValueMetadataReader{runtime: &rt.runtime}, id, lexical, resolve, needs)
}

type publishedSimpleValueMetadataReader struct {
	runtime *schemaRuntime
}

func (r publishedSimpleValueMetadataReader) simpleValueType(id SimpleTypeID) (SimpleValueType, bool) {
	route, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id)
	if !ok {
		return SimpleValueType{}, false
	}
	cold, ok := r.runtime.SimpleValueCold.read(id)
	if !ok {
		return SimpleValueType{}, false
	}
	return simpleValueTypeForRouteAndCold(route, cold), true
}

func (r publishedSimpleValueMetadataReader) simpleValueFacets(id SimpleTypeID) (SimpleValueFacets, bool) {
	if _, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id); !ok {
		return SimpleValueFacets{}, false
	}
	cold, ok := r.runtime.SimpleValueCold.read(id)
	if !ok {
		return SimpleValueFacets{}, false
	}
	return simpleValueFacetsForColdRead(cold), true
}

func (r publishedSimpleValueMetadataReader) simpleValueStringEnumeration(id SimpleTypeID, canonical string) (bool, bool) {
	if _, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id); !ok {
		return false, false
	}
	cold, ok := r.runtime.SimpleValueCold.read(id)
	if !ok {
		return false, false
	}
	if cold == nil {
		return false, true
	}
	for _, literal := range cold.enumeration {
		if literal.canonical == canonical {
			return true, true
		}
	}
	return false, true
}

func (r publishedSimpleValueMetadataReader) simpleValueNotation(ns, local string) (bool, bool) {
	return r.runtime.Notations[ExpandedName{Namespace: ns, Local: local}], true
}

func (publishedSimpleValueMetadataReader) simpleValueUnsupported(err error) bool {
	return xsderrors.IsUnsupported(err)
}

func (rt *Schema) validatePublishedRawSimpleValue(id SimpleTypeID, raw []byte) (bool, error) {
	return validateResolvedRawSimpleValue(rawSimpleValueResolver{runtime: &rt.runtime}, id, raw)
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
