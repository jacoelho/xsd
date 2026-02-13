package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/typewalk"
	"github.com/jacoelho/xsd/internal/whitespace"
)

func (r *typeResolver) builtinNameForType(typ model.Type) (model.TypeName, bool) {
	var (
		out   model.TypeName
		found bool
	)
	typewalk.Walk(typ, r.nextType, func(current model.Type) bool {
		if bt := builtinForType(current); bt != nil {
			out = model.TypeName(bt.Name().Local)
			found = true
			return false
		}
		_, ok := model.AsSimpleType(current)
		return ok
	})
	return out, found
}

func (r *typeResolver) isQNameOrNotation(typ model.Type) bool {
	result := false
	typewalk.Walk(typ, r.nextType, func(current model.Type) bool {
		if bt := builtinForType(current); bt != nil {
			result = model.IsQNameOrNotation(bt.Name())
			return false
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			result = false
			return false
		}
		if r.variety(st) != model.AtomicVariety {
			result = false
			return false
		}
		if model.IsQNameOrNotation(st.Name()) {
			result = true
			return false
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			base := st.Restriction.Base
			if (base.Namespace == model.XSDNamespace || base.Namespace == "") &&
				(base.Local == string(model.TypeNameQName) || base.Local == string(model.TypeNameNOTATION)) {
				result = true
				return false
			}
		}
		return true
	})
	return result
}

func (r *typeResolver) isIntegerDerived(typ model.Type) bool {
	result := false
	typewalk.Walk(typ, r.nextType, func(current model.Type) bool {
		if bt := builtinForType(current); bt != nil {
			result = isIntegerTypeName(bt.Name().Local)
			return false
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			result = false
			return false
		}
		if r.variety(st) != model.AtomicVariety {
			result = false
			return false
		}
		if isIntegerTypeName(st.Name().Local) {
			result = true
			return false
		}
		return true
	})
	return result
}

func (r *typeResolver) whitespaceMode(typ model.Type) runtime.WhitespaceMode {
	mode := runtime.WSPreserve
	typewalk.Walk(typ, r.nextType, func(current model.Type) bool {
		if bt := builtinForType(current); bt != nil {
			mode = whitespace.ToRuntime(bt.WhiteSpace())
			return false
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			mode = runtime.WSPreserve
			return false
		}
		if st.WhiteSpaceExplicit() || st.List != nil || st.Union != nil {
			mode = whitespace.ToRuntime(st.WhiteSpace())
			return false
		}
		mode = whitespace.ToRuntime(st.WhiteSpace())
		return true
	})
	return mode
}

func (r *typeResolver) primitiveName(typ model.Type) (string, error) {
	if typ == nil {
		return "", fmt.Errorf("missing type")
	}
	if r.varietyForType(typ) != model.AtomicVariety {
		return "", fmt.Errorf("primitive type undefined for %s", typ.Name().Local)
	}
	return r.primitiveNameAtomic(typ, make(map[model.Type]bool))
}

func (r *typeResolver) primitiveNameAtomic(typ model.Type, seen map[model.Type]bool) (string, error) {
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
	st, ok := model.AsSimpleType(typ)
	if !ok {
		return "", fmt.Errorf("unsupported type")
	}
	if st.IsBuiltin() {
		if builtin := builtins.Get(model.TypeName(st.Name().Local)); builtin != nil {
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
