package runtime

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/jacoelho/xsd/xsderrors"
)

type publishedSimpleValueMetadataReader struct {
	runtime *schemaRuntime
}

func (r publishedSimpleValueMetadataReader) simpleValueType(id SimpleTypeID) (SimpleValueType, bool) {
	route, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id)
	if !ok {
		return SimpleValueType{}, false
	}
	cold, ok := r.runtime.SimpleTypeCold.read(id)
	if !ok {
		return SimpleValueType{}, false
	}
	return simpleValueTypeForRouteAndCold(route, cold), true
}

func (r publishedSimpleValueMetadataReader) simpleValueFacets(id SimpleTypeID) (SimpleValueFacets, bool) {
	if _, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id); !ok {
		return SimpleValueFacets{}, false
	}
	cold, ok := r.runtime.SimpleTypeCold.read(id)
	if !ok {
		return SimpleValueFacets{}, false
	}
	return simpleValueFacetsForColdRead(cold), true
}

func (r publishedSimpleValueMetadataReader) simpleValueStringEnumeration(id SimpleTypeID, canonical string) (bool, bool) {
	if _, ok := simpleValueRouteReadByID(r.runtime.SimpleValueRoutes, id); !ok {
		return false, false
	}
	cold, ok := r.runtime.SimpleTypeCold.read(id)
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

func validateRawStringLength(raw []byte, whitespace WhitespaceMode, facets LengthFacetValues) error {
	count, err := normalizedRawStringLength(raw, whitespace)
	if err != nil {
		return err
	}
	return ValidateLengthFacets(facets, count)
}

type rawStringLengthState struct {
	count        uint64
	seen         bool
	pendingSpace bool
}

func normalizedRawStringLength(raw []byte, whitespace WhitespaceMode) (uint32, error) {
	var state rawStringLengthState
	for len(raw) != 0 {
		r, size := utf8.DecodeRune(raw)
		if r == utf8.RuneError && size == 1 {
			return 0, errors.New("invalid UTF-8 string")
		}
		raw = raw[size:]
		if err := state.appendRune(r, whitespace); err != nil {
			return 0, err
		}
	}
	return uint32(state.count), nil //nolint:gosec // appendRune rejects counts above uint32.
}

func (s *rawStringLengthState) appendRune(r rune, whitespace WhitespaceMode) error {
	if whitespace == WhitespaceCollapse && isXMLWhitespaceRune(r) {
		if s.seen {
			s.pendingSpace = true
		}
		return nil
	}
	if s.pendingSpace {
		s.count++
		s.pendingSpace = false
	}
	s.count++
	s.seen = true
	if s.count > math.MaxUint32 {
		return fmt.Errorf("string length exceeds %d", uint64(math.MaxUint32))
	}
	return nil
}

func isXMLWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
