package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// FacetSource is the raw source information needed to admit a simple-type
// facet child.
type FacetSource struct {
	Local          string
	InXSDNamespace bool
	HasValue       bool
	Variety        runtime.SimpleVariety
	Primitive      runtime.PrimitiveKind
}

// IsFacetLocal reports whether local is one of the XSD facet element names.
func IsFacetLocal(local string) bool {
	_, ok := facetMaskForLocal(local)
	return ok
}

// ValidateFacetSource validates a facet child and reports whether schema
// compilation should compile it. Non-XSD non-facet children are skipped.
func ValidateFacetSource(source FacetSource) (bool, error) {
	mask, ok := facetMaskForLocal(source.Local)
	if !ok {
		if source.InXSDNamespace {
			return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "unsupported facet "+source.Local)
		}
		return false, nil
	}
	if !source.HasValue {
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, source.Local+" missing value")
	}
	if !runtime.FacetAllowedForSimpleType(source.Variety, source.Primitive, mask) {
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "facet "+source.Local+" is not allowed")
	}
	return true, nil
}

// ParseWhitespaceFacetValue parses and validates an xs:whiteSpace facet value
// against the base simple type's whitespace mode.
func ParseWhitespaceFacetValue(value string, base runtime.WhitespaceMode) (runtime.WhitespaceMode, error) {
	var mode runtime.WhitespaceMode
	switch value {
	case vocab.XSDWhitespacePreserve:
		mode = runtime.WhitespacePreserve
	case vocab.XSDWhitespaceReplace:
		mode = runtime.WhitespaceReplace
	case vocab.XSDWhitespaceCollapse:
		mode = runtime.WhitespaceCollapse
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "invalid whiteSpace facet "+value)
	}
	if !runtime.ValidWhitespaceRestriction(base, mode) {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "whiteSpace cannot loosen base whiteSpace")
	}
	return mode, nil
}

func facetMaskForLocal(local string) (runtime.FacetMask, bool) {
	switch local {
	case vocab.XSDFacetLength:
		return runtime.FacetLength, true
	case vocab.XSDFacetMinLength:
		return runtime.FacetMinLength, true
	case vocab.XSDFacetMaxLength:
		return runtime.FacetMaxLength, true
	case vocab.XSDFacetTotalDigits:
		return runtime.FacetTotalDigits, true
	case vocab.XSDFacetFractionDigits:
		return runtime.FacetFractionDigits, true
	case vocab.XSDFacetMinInclusive:
		return runtime.FacetMinInclusive, true
	case vocab.XSDFacetMaxInclusive:
		return runtime.FacetMaxInclusive, true
	case vocab.XSDFacetMinExclusive:
		return runtime.FacetMinExclusive, true
	case vocab.XSDFacetMaxExclusive:
		return runtime.FacetMaxExclusive, true
	case vocab.XSDFacetEnumeration:
		return runtime.FacetEnumeration, true
	case vocab.XSDFacetPattern:
		return runtime.FacetPattern, true
	case vocab.XSDFacetWhiteSpace:
		return runtime.FacetWhiteSpace, true
	default:
		return 0, false
	}
}
