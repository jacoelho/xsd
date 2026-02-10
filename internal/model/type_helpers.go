package model

// isQNameOrNotationType reports whether the type represents xs:QName or xs:NOTATION.
func isQNameOrNotationType(typ Type) bool {
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
