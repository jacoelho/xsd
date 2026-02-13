package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

// areFieldTypesCompatible checks if two field types are compatible for keyref schemacheck.
// Types are compatible if:
// 1. They are identical
// 2. One is derived from the other
// 3. Both derive from the same primitive type
func areFieldTypesCompatible(field1Type, field2Type types.Type) bool {
	if field1Type == nil || field2Type == nil {
		return false
	}

	if field1Type == field2Type {
		return true
	}
	name1 := field1Type.Name()
	name2 := field2Type.Name()
	if !name1.IsZero() && !name2.IsZero() && name1 == name2 {
		return true
	}

	if types.IsDerivedFrom(field1Type, field2Type) {
		return true
	}
	if types.IsDerivedFrom(field2Type, field1Type) {
		return true
	}

	prim1 := getPrimitiveType(field1Type)
	prim2 := getPrimitiveType(field2Type)
	if prim1 != nil && prim2 != nil && prim1.Name() == prim2.Name() {
		return true
	}

	return false
}

// getPrimitiveType returns the primitive type for a given type.
func getPrimitiveType(typ types.Type) types.Type {
	if typ == nil {
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok {
		return st.PrimitiveType()
	}

	primitive := typ.PrimitiveType()
	if primitive != nil {
		return primitive
	}

	return nil
}
