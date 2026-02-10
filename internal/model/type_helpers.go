package model

// IsQNameOrNotationType reports whether the type represents xs:QName or xs:NOTATION.
func IsQNameOrNotationType(typ Type) bool {
	if typ == nil {
		return false
	}
	switch t := typ.(type) {
	case *SimpleType:
		return t.IsQNameOrNotationType()
	default:
		return IsQNameOrNotation(typ.Name())
	}
}
