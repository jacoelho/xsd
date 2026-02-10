package listtypes

var builtinListItemsByName = map[string]string{
	"NMTOKENS": "NMTOKEN",
	"IDREFS":   "IDREF",
	"ENTITIES": "ENTITY",
}

// IsTypeName reports whether name is a built-in list datatype name.
func IsTypeName(name string) bool {
	_, ok := ItemTypeName(name)
	return ok
}

// ItemTypeName returns the built-in item type name for name.
func ItemTypeName(name string) (string, bool) {
	item, ok := builtinListItemsByName[name]
	return item, ok
}
