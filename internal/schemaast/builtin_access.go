package schemaast

// IsBuiltin reports whether a QName resolves to a built-in type.
func IsBuiltin(qname QName) bool {
	if qname.Namespace != XSDNamespace {
		return false
	}
	_, ok := knownBuiltinTypeNames[TypeName(qname.Local)]
	return ok
}

// IsBuiltinListTypeName reports whether name is one of the built-in list simple
func IsBuiltinListTypeName(name string) bool {
	return isBuiltinListTypeName(name)
}

// BuiltinListItemTypeName returns the built-in item type for a built-in list type.
func BuiltinListItemTypeName(name string) (TypeName, bool) {
	return builtinListItemTypeName(name)
}

var knownBuiltinTypeNames = map[TypeName]struct{}{
	TypeNameAnyType:            {},
	TypeNameAnySimpleType:      {},
	TypeNameString:             {},
	TypeNameBoolean:            {},
	TypeNameDecimal:            {},
	TypeNameFloat:              {},
	TypeNameDouble:             {},
	TypeNameDuration:           {},
	TypeNameDateTime:           {},
	TypeNameTime:               {},
	TypeNameDate:               {},
	TypeNameGYearMonth:         {},
	TypeNameGYear:              {},
	TypeNameGMonthDay:          {},
	TypeNameGDay:               {},
	TypeNameGMonth:             {},
	TypeNameHexBinary:          {},
	TypeNameBase64Binary:       {},
	TypeNameAnyURI:             {},
	TypeNameQName:              {},
	TypeNameNOTATION:           {},
	TypeNameNormalizedString:   {},
	TypeNameToken:              {},
	TypeNameLanguage:           {},
	TypeNameName:               {},
	TypeNameNCName:             {},
	TypeNameID:                 {},
	TypeNameIDREF:              {},
	TypeNameIDREFS:             {},
	TypeNameENTITY:             {},
	TypeNameENTITIES:           {},
	TypeNameNMTOKEN:            {},
	TypeNameNMTOKENS:           {},
	TypeNameInteger:            {},
	TypeNameLong:               {},
	TypeNameInt:                {},
	TypeNameShort:              {},
	TypeNameByte:               {},
	TypeNameNonNegativeInteger: {},
	TypeNamePositiveInteger:    {},
	TypeNameUnsignedLong:       {},
	TypeNameUnsignedInt:        {},
	TypeNameUnsignedShort:      {},
	TypeNameUnsignedByte:       {},
	TypeNameNonPositiveInteger: {},
	TypeNameNegativeInteger:    {},
}
