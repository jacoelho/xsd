package model

// ResolveSimpleContentBaseType returns the first non-simpleContent base type.
func ResolveSimpleContentBaseType(typ Type) Type {
	current := typ
	for current != nil {
		ct, ok := as[*ComplexType](current)
		if !ok {
			return current
		}
		if _, ok := ct.Content().(*SimpleContent); !ok {
			return current
		}
		next := ct.BaseType()
		if next == nil || next == current {
			return current
		}
		current = next
	}
	return current
}
