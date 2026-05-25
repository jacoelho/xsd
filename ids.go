package xsd

import "math"

const (
	maxUint32Value = math.MaxUint32
	maxUint32Text  = "4294967295"
)

// checkedUint32 centralizes int-to-uint32 boundary checks while preserving the
// caller's domain-specific limit error.
func checkedUint32(n int, msg string) (uint32, error) {
	if n < 0 || uint64(n) > maxUint32Value {
		return 0, schemaCompile(ErrSchemaLimit, msg)
	}
	return uint32(n), nil
}

func saturatingUint32(n int) uint32 {
	if n < 0 || uint64(n) > maxUint32Value {
		return maxUint32Value
	}
	return uint32(n)
}

func nextNamespaceID(n int) (namespaceID, error) {
	id, err := checkedUint32(n, "schema namespace limit exceeded")
	return namespaceID(id), err
}

func nextLocalNameID(n int) (localNameID, error) {
	id, err := checkedUint32(n, "schema local-name limit exceeded")
	return localNameID(id), err
}

func nextSimpleTypeID(n int) (simpleTypeID, error) {
	id, err := checkedUint32(n, "simple type limit exceeded")
	return simpleTypeID(id), err
}

func nextComplexTypeID(n int) (complexTypeID, error) {
	id, err := checkedUint32(n, "complex type limit exceeded")
	return complexTypeID(id), err
}

func nextElementID(n int) (elementID, error) {
	id, err := checkedUint32(n, "element declaration limit exceeded")
	return elementID(id), err
}

func nextAttributeID(n int) (attributeID, error) {
	id, err := checkedUint32(n, "attribute declaration limit exceeded")
	return attributeID(id), err
}

func nextContentModelID(n int) (contentModelID, error) {
	id, err := checkedUint32(n, "content model limit exceeded")
	return contentModelID(id), err
}

func nextAttributeUseSetID(n int) (attributeUseSetID, error) {
	id, err := checkedUint32(n, "attribute use set limit exceeded")
	return attributeUseSetID(id), err
}

func nextWildcardID(n int) (wildcardID, error) {
	id, err := checkedUint32(n, "wildcard limit exceeded")
	return wildcardID(id), err
}

func nextIdentityConstraintID(n int) (identityConstraintID, error) {
	id, err := checkedUint32(n, "identity constraint limit exceeded")
	return identityConstraintID(id), err
}
