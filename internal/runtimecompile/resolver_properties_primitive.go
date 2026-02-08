package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

func (r *typeResolver) primitiveName(typ types.Type) (string, error) {
	if typ == nil {
		return "", fmt.Errorf("missing type")
	}
	if r.varietyForType(typ) != types.AtomicVariety {
		return "", fmt.Errorf("primitive type undefined for %s", typ.Name().Local)
	}
	return r.primitiveNameAtomic(typ, make(map[types.Type]bool))
}

func (r *typeResolver) primitiveNameAtomic(typ types.Type, seen map[types.Type]bool) (string, error) {
	if typ == nil {
		return "", fmt.Errorf("missing type")
	}
	if seen[typ] {
		return "", fmt.Errorf("primitive type cycle")
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		primitive := bt.PrimitiveType()
		if primitive == nil {
			return "", fmt.Errorf("primitive type not found")
		}
		return primitive.Name().Local, nil
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return "", fmt.Errorf("unsupported type")
	}
	if st.IsBuiltin() {
		if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
			primitive := builtin.PrimitiveType()
			if primitive == nil {
				return "", fmt.Errorf("primitive type not found")
			}
			return primitive.Name().Local, nil
		}
	}
	if base := r.baseType(st); base != nil {
		return r.primitiveNameAtomic(base, seen)
	}
	return "", fmt.Errorf("primitive type not found")
}
