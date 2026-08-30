package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// CheckedUint32Index returns n as a uint32 index with a schema-limit diagnostic.
func CheckedUint32Index(n int, msg string) (uint32, error) {
	id, ok := runtime.NewUint32Index(n)
	if !ok {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
	}
	return id, nil
}

// NextSimpleTypeID returns the next simple-type ID with a schema-limit diagnostic.
func NextSimpleTypeID(n int) (runtime.SimpleTypeID, error) {
	id, ok := runtime.NextSimpleTypeID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple type limit exceeded")
	}
	return id, nil
}

// NextComplexTypeID returns the next complex-type ID with a schema-limit diagnostic.
func NextComplexTypeID(n int) (runtime.ComplexTypeID, error) {
	id, ok := runtime.NextComplexTypeID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "complex type limit exceeded")
	}
	return id, nil
}

// NextElementID returns the next element ID with a schema-limit diagnostic.
func NextElementID(n int) (runtime.ElementID, error) {
	id, ok := runtime.NextElementID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "element declaration limit exceeded")
	}
	return id, nil
}

// NextAttributeID returns the next attribute ID with a schema-limit diagnostic.
func NextAttributeID(n int) (runtime.AttributeID, error) {
	id, ok := runtime.NextAttributeID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "attribute declaration limit exceeded")
	}
	return id, nil
}

// NextContentModelID returns the next content-model ID with a schema-limit diagnostic.
func NextContentModelID(n int) (runtime.ContentModelID, error) {
	id, ok := runtime.NextContentModelID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model limit exceeded")
	}
	return id, nil
}

// NextAttributeUseSetID returns the next attribute-use-set ID with a schema-limit diagnostic.
func NextAttributeUseSetID(n int) (runtime.AttributeUseSetID, error) {
	id, ok := runtime.NextAttributeUseSetID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "attribute use set limit exceeded")
	}
	return id, nil
}

// NextWildcardID returns the next wildcard ID with a schema-limit diagnostic.
func NextWildcardID(n int) (runtime.WildcardID, error) {
	id, ok := runtime.NextWildcardID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "wildcard limit exceeded")
	}
	return id, nil
}

// NextIdentityConstraintID returns the next identity-constraint ID with a schema-limit diagnostic.
func NextIdentityConstraintID(n int) (runtime.IdentityConstraintID, error) {
	id, ok := runtime.NextIdentityConstraintID(n)
	if !ok {
		return id, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "identity constraint limit exceeded")
	}
	return id, nil
}
