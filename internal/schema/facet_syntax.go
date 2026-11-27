package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// facetSource is the source information needed to admit a facet element on a
// simple type.
type facetSource struct {
	Local     string
	Variety   valuepkg.Variety
	Primitive valuepkg.PrimitiveKind
}

// isFacetLocal reports whether local is one of the XSD facet element names.
func isFacetLocal(local string) bool {
	_, ok := facetMaskForLocal(local)
	return ok
}

// validateFacetSource returns the facet that source, a facet element,
// declares on its simple type, or an error when the type does not admit that
// facet.
func validateFacetSource(source facetSource) (valuepkg.FacetMask, error) {
	mask, ok := facetMaskForLocal(source.Local)
	if !ok {
		return 0, xsderrors.InternalInvariant("facet source is not a facet element", nil)
	}
	if !valuepkg.FacetAllowed(source.Variety, source.Primitive, mask) {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "facet "+facetLabel(source.Local)+" is not allowed", nil)
	}
	return mask, nil
}

// facetValue is what the start tag of a facet admits from its value and
// fixed attributes: the size of a size facet, the mode of xs:whiteSpace, the
// compiled pattern of xs:pattern, and fixed. The facet's typed source record
// stores it and compilation reads it.
type facetValue struct {
	pattern    *xsdregex.Pattern
	size       uint32
	whitespace valuepkg.WhitespaceMode
	fixed      bool
}

// admitFacetValue parses value, the value attribute of the facet element
// named element, when the element is a size facet, xs:whiteSpace or
// xs:pattern, and is the only owner of their value syntax and limits. A value
// it rejects is a violation of the facet's start tag, whatever the base type
// of the restriction. Every other facet value depends on the base type and is
// compiled with it. A pattern is admitted through patterns, which compiles
// each distinct text once and charges MaxSchemaPatternAnalysisSteps and
// MaxSchemaPatternRetainedBytes.
func admitFacetValue(element, value string, patterns *schemaPatterns) (facetValue, error) {
	switch element {
	case vocab.XSDFacetPattern:
		pattern, err := patterns.admit(value)
		return facetValue{pattern: pattern}, err
	case vocab.XSDFacetLength, vocab.XSDFacetMinLength, vocab.XSDFacetMaxLength, vocab.XSDFacetTotalDigits, vocab.XSDFacetFractionDigits:
		size, err := parseSizeFacetValue(element, value)
		return facetValue{size: size}, err
	case vocab.XSDFacetWhiteSpace:
		mode, err := parseWhitespaceFacetValue(value)
		return facetValue{whitespace: mode}, err
	}
	return facetValue{}, nil
}

// parseWhitespaceFacetValue parses an xs:whiteSpace facet value. Loosening the
// base whiteSpace is rejected by value.Builder during derivation.
func parseWhitespaceFacetValue(value string) (valuepkg.WhitespaceMode, error) {
	value = lex.CollapseXMLWhitespace(value)
	var mode valuepkg.WhitespaceMode
	switch value {
	case vocab.XSDWhitespacePreserve:
		mode = valuepkg.WhitespacePreserve
	case vocab.XSDWhitespaceReplace:
		mode = valuepkg.WhitespaceReplace
	case vocab.XSDWhitespaceCollapse:
		mode = valuepkg.WhitespaceCollapse
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "invalid whiteSpace facet "+diagnostictext.Quote(value), nil)
	}
	return mode, nil
}

func facetMaskForLocal(local string) (valuepkg.FacetMask, bool) {
	switch local {
	case vocab.XSDFacetLength:
		return valuepkg.FacetLength, true
	case vocab.XSDFacetMinLength:
		return valuepkg.FacetMinLength, true
	case vocab.XSDFacetMaxLength:
		return valuepkg.FacetMaxLength, true
	case vocab.XSDFacetTotalDigits:
		return valuepkg.FacetTotalDigits, true
	case vocab.XSDFacetFractionDigits:
		return valuepkg.FacetFractionDigits, true
	case vocab.XSDFacetMinInclusive:
		return valuepkg.FacetMinInclusive, true
	case vocab.XSDFacetMaxInclusive:
		return valuepkg.FacetMaxInclusive, true
	case vocab.XSDFacetMinExclusive:
		return valuepkg.FacetMinExclusive, true
	case vocab.XSDFacetMaxExclusive:
		return valuepkg.FacetMaxExclusive, true
	case vocab.XSDFacetEnumeration:
		return valuepkg.FacetEnumeration, true
	case vocab.XSDFacetPattern:
		return valuepkg.FacetPattern, true
	case vocab.XSDFacetWhiteSpace:
		return valuepkg.FacetWhiteSpace, true
	default:
		return 0, false
	}
}
