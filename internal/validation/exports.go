package validation

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateElementDeclStructure exposes element declaration structure validation for tests.
func ValidateElementDeclStructure(schema *parser.Schema, qname types.QName, decl *types.ElementDecl) error {
	return validateElementDeclStructure(schema, qname, decl)
}

// ResolveTypeReference exposes type reference resolution for validation callers.
func ResolveTypeReference(schema *parser.Schema, typ types.Type, allowMissing bool) types.Type {
	return resolveTypeReference(schema, typ, allowMissing)
}

// ResolveTypeForValidation resolves a type reference for validation callers.
func ResolveTypeForValidation(schema *parser.Schema, typ types.Type) types.Type {
	return resolveTypeReference(schema, typ, false)
}

// ValidateFacetConstraints exposes facet constraint validation for tests.
func ValidateFacetConstraints(facetList []types.Facet, baseType types.Type, baseQName types.QName) error {
	return validateFacetConstraints(facetList, baseType, baseQName)
}

// ResolveFieldType exposes field XPath type resolution for reference validation.
func ResolveFieldType(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string) (types.Type, error) {
	return resolveFieldType(schema, field, constraintElement, selectorXPath)
}

// ElementTypesCompatible exposes element type compatibility checks.
func ElementTypesCompatible(a, b types.Type) bool {
	return elementTypesCompatible(a, b)
}

// ResolveSelectorElementType exposes selector XPath type resolution.
func ResolveSelectorElementType(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string) (types.Type, error) {
	return resolveSelectorElementType(schema, constraintElement, selectorXPath)
}

// CollectAllElementDeclarationsFromType exposes element collection from complex types.
func CollectAllElementDeclarationsFromType(schema *parser.Schema, ct *types.ComplexType) []*types.ElementDecl {
	return collectAllElementDeclarationsFromType(schema, ct)
}

// IsIDOnlyType exposes ID-only type checks for defaults and fixed values.
func IsIDOnlyType(qname types.QName) bool {
	return isIDOnlyType(qname)
}

// IsIDOnlyDerivedType exposes ID-only derived type checks.
func IsIDOnlyDerivedType(st *types.SimpleType) bool {
	return isIDOnlyDerivedType(st)
}

// ResolveSimpleTypeReference exposes simple type lookups by QName.
func ResolveSimpleTypeReference(schema *parser.Schema, qname types.QName) types.Type {
	return resolveSimpleTypeReference(schema, qname)
}

// ValidateRangeFacets exposes range facet consistency validation.
func ValidateRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseTypeName string, bt *types.BuiltinType) error {
	return validateRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive, baseTypeName, bt)
}

// CompareNumericOrString exposes numeric/string comparison used by range facets.
func CompareNumericOrString(v1, v2, baseTypeName string, bt *types.BuiltinType) int {
	return compareNumericOrString(v1, v2, baseTypeName, bt)
}

// ValidateSelectorXPath exposes selector XPath validation.
func ValidateSelectorXPath(xpath string) error {
	return validateSelectorXPath(xpath)
}

// ValidateFieldXPath exposes field XPath validation.
func ValidateFieldXPath(xpath string) error {
	return validateFieldXPath(xpath)
}

// ValidateNoCircularDerivation exposes cycle checks for complex type derivations.
func ValidateNoCircularDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	return validateNoCircularDerivation(schema, ct)
}

// ValidateMixedContentDerivation exposes mixed content derivation checks.
func ValidateMixedContentDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	return validateMixedContentDerivation(schema, ct)
}
