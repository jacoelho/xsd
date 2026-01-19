package schemacheck

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// TypeReferencePolicy controls how missing type references are handled.
type TypeReferencePolicy int

const (
	// TypeReferenceMustExist requires referenced types to be resolved.
	TypeReferenceMustExist TypeReferencePolicy = iota
	// TypeReferenceAllowMissing allows unresolved placeholders to pass through.
	TypeReferenceAllowMissing
)

// ResolveTypeReference resolves a type reference for validation callers.
func ResolveTypeReference(schema *parser.Schema, typ types.Type, policy TypeReferencePolicy) types.Type {
	return resolveTypeReference(schema, typ, policy)
}

// ResolveFieldType resolves a field XPath to its selected type.
func ResolveFieldType(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (types.Type, error) {
	return resolveFieldType(schema, field, constraintElement, selectorXPath, nsContext)
}

// ResolveFieldElementDecl resolves a field XPath to the selected element declaration.
func ResolveFieldElementDecl(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	return resolveFieldElementDecl(schema, field, constraintElement, selectorXPath, nsContext)
}

// ElementTypesCompatible reports whether two element declaration types are consistent.
func ElementTypesCompatible(a, b types.Type) bool {
	return elementTypesCompatible(a, b)
}

// ResolveSelectorElementType resolves a selector XPath to its element type.
func ResolveSelectorElementType(schema *parser.Schema, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (types.Type, error) {
	return resolveSelectorElementType(schema, constraintElement, selectorXPath, nsContext)
}

// CollectAllElementDeclarationsFromType collects element declarations from a complex type.
func CollectAllElementDeclarationsFromType(schema *parser.Schema, ct *types.ComplexType) []*types.ElementDecl {
	return collectAllElementDeclarationsFromType(schema, ct)
}

// IsIDOnlyType reports whether a QName is an ID-only type.
func IsIDOnlyType(qname types.QName) bool {
	return isIDOnlyType(qname)
}

// IsIDOnlyDerivedType reports whether a simple type is derived from ID only.
func IsIDOnlyDerivedType(st *types.SimpleType) bool {
	return isIDOnlyDerivedType(st)
}

// ResolveSimpleTypeReference resolves a simple type by QName.
func ResolveSimpleTypeReference(schema *parser.Schema, qname types.QName) types.Type {
	return resolveSimpleTypeReference(schema, qname)
}
