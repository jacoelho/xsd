package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	valuepkg "github.com/jacoelho/xsd/internal/value"

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
		return 0, xsderrors.InternalInvariant("facet source is not a facet element")
	}
	if !valuepkg.FacetAllowed(source.Variety, source.Primitive, mask) {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "facet "+facetLabel(source.Local)+" is not allowed")
	}
	return mask, nil
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
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "invalid whiteSpace facet "+diagnostictext.Quote(value))
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
