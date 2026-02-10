package model

// AnyTypeQName returns the canonical QName for xs:anyType.
func AnyTypeQName() QName {
	return QName{Namespace: XSDNamespace, Local: string(TypeNameAnyType)}
}

// AnySimpleTypeQName returns the canonical QName for xs:anySimpleType.
func AnySimpleTypeQName() QName {
	return QName{Namespace: XSDNamespace, Local: string(TypeNameAnySimpleType)}
}

// IsAnyTypeQName reports whether qname is xs:anyType.
func IsAnyTypeQName(qname QName) bool {
	return qname == AnyTypeQName()
}

// IsAnySimpleTypeQName reports whether qname is xs:anySimpleType.
func IsAnySimpleTypeQName(qname QName) bool {
	return qname == AnySimpleTypeQName()
}
