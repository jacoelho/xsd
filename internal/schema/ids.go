package schema

import (
	"github.com/jacoelho/xsd/xsderrors"
)

// checkedUint32Index returns n as a uint32 index with a schema-limit diagnostic.
func checkedUint32Index(n int, msg string) (uint32, error) {
	id, ok := newUint32Index(n)
	if !ok {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
	}
	return id, nil
}

// nextSimpleTypeID returns the next simple-type ID with a schema-limit diagnostic.
func nextSimpleTypeID(n int) (SimpleTypeID, error) {
	raw, ok := newRuntimeID(n)
	id := SimpleTypeID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple type limit exceeded")
	}
	return id, nil
}

// nextComplexTypeID returns the next complex-type ID with a schema-limit diagnostic.
func nextComplexTypeID(n int) (ComplexTypeID, error) {
	raw, ok := newRuntimeID(n)
	id := ComplexTypeID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "complex type limit exceeded")
	}
	return id, nil
}

// nextElementID returns the next element ID with a schema-limit diagnostic.
func nextElementID(n int) (ElementID, error) {
	raw, ok := newRuntimeID(n)
	id := ElementID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "element declaration limit exceeded")
	}
	return id, nil
}

// nextAttributeID returns the next attribute ID with a schema-limit diagnostic.
func nextAttributeID(n int) (AttributeID, error) {
	raw, ok := newRuntimeID(n)
	id := AttributeID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "attribute declaration limit exceeded")
	}
	return id, nil
}

// nextContentModelID returns the next content-model ID with a schema-limit diagnostic.
func nextContentModelID(n int) (contentModelID, error) {
	raw, ok := newRuntimeID(n)
	id := contentModelID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model limit exceeded")
	}
	return id, nil
}

// nextAttributeUseSetID returns the next attribute-use-set ID with a schema-limit diagnostic.
func nextAttributeUseSetID(n int) (attributeUseSetID, error) {
	raw, ok := newRuntimeID(n)
	id := attributeUseSetID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "attribute use set limit exceeded")
	}
	return id, nil
}

// nextWildcardID returns the next wildcard ID with a schema-limit diagnostic.
func nextWildcardID(n int) (WildcardID, error) {
	raw, ok := newRuntimeID(n)
	id := WildcardID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "wildcard limit exceeded")
	}
	return id, nil
}

// nextIdentityConstraintID returns the next identity-constraint ID with a schema-limit diagnostic.
func nextIdentityConstraintID(n int) (IdentityConstraintID, error) {
	raw, ok := newRuntimeID(n)
	id := IdentityConstraintID(raw)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "identity constraint limit exceeded")
	}
	return id, nil
}
