package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func runtimeIDError(ok bool, msg string) error {
	if !ok {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
	}
	return nil
}

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
	return id, runtimeIDError(ok, "simple type limit exceeded")
}

// NextComplexTypeID returns the next complex-type ID with a schema-limit diagnostic.
func NextComplexTypeID(n int) (runtime.ComplexTypeID, error) {
	id, ok := runtime.NextComplexTypeID(n)
	return id, runtimeIDError(ok, "complex type limit exceeded")
}

// NextElementID returns the next element ID with a schema-limit diagnostic.
func NextElementID(n int) (runtime.ElementID, error) {
	id, ok := runtime.NextElementID(n)
	return id, runtimeIDError(ok, "element declaration limit exceeded")
}

// NextAttributeID returns the next attribute ID with a schema-limit diagnostic.
func NextAttributeID(n int) (runtime.AttributeID, error) {
	id, ok := runtime.NextAttributeID(n)
	return id, runtimeIDError(ok, "attribute declaration limit exceeded")
}

// NextContentModelID returns the next content-model ID with a schema-limit diagnostic.
func NextContentModelID(n int) (runtime.ContentModelID, error) {
	id, ok := runtime.NextContentModelID(n)
	return id, runtimeIDError(ok, "content model limit exceeded")
}

// NextAttributeUseSetID returns the next attribute-use-set ID with a schema-limit diagnostic.
func NextAttributeUseSetID(n int) (runtime.AttributeUseSetID, error) {
	id, ok := runtime.NextAttributeUseSetID(n)
	return id, runtimeIDError(ok, "attribute use set limit exceeded")
}

// NextWildcardID returns the next wildcard ID with a schema-limit diagnostic.
func NextWildcardID(n int) (runtime.WildcardID, error) {
	id, ok := runtime.NextWildcardID(n)
	return id, runtimeIDError(ok, "wildcard limit exceeded")
}

// NextIdentityConstraintID returns the next identity-constraint ID with a schema-limit diagnostic.
func NextIdentityConstraintID(n int) (runtime.IdentityConstraintID, error) {
	id, ok := runtime.NextIdentityConstraintID(n)
	return id, runtimeIDError(ok, "identity constraint limit exceeded")
}
