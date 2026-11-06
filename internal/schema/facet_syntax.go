package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	valuepkg "github.com/jacoelho/xsd/internal/value"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// facetSource is the raw source information needed to admit a simple-type
// facet child.
type facetSource struct {
	Local          string
	InXSDNamespace bool
	HasValue       bool
	Variety        valuepkg.Variety
	Primitive      valuepkg.PrimitiveKind
}

// isFacetLocal reports whether local is one of the XSD facet element names.
func isFacetLocal(local string) bool {
	_, ok := facetMaskForLocal(local)
	return ok
}

// validateFacetSource validates a facet child and reports whether schema
// compilation should compile it. Non-XSD non-facet children are skipped.
func validateFacetSource(source facetSource) (bool, error) {
	mask, ok := facetMaskForLocal(source.Local)
	if !ok {
		if source.InXSDNamespace {
			return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "unsupported facet "+diagnostictext.Quote(source.Local))
		}
		return false, nil
	}
	if !source.HasValue {
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, facetLabel(source.Local)+" missing value")
	}
	if !valuepkg.FacetAllowed(source.Variety, source.Primitive, mask) {
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "facet "+facetLabel(source.Local)+" is not allowed")
	}
	return true, nil
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
