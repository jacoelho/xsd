package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func (r *typeResolver) builtinNameForType(typ model.Type) (model.TypeName, bool) {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) (model.TypeName, bool)
	walk = func(current model.Type) (model.TypeName, bool) {
		if current == nil {
			return "", false
		}
		if seen[current] {
			return "", false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return model.TypeName(bt.Name().Local), true
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			return "", false
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return "", false
	}
	return walk(typ)
}

func (r *typeResolver) isQNameOrNotation(typ model.Type) bool {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) bool
	walk = func(current model.Type) bool {
		if current == nil {
			return false
		}
		if seen[current] {
			return false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return model.IsQNameOrNotation(bt.Name())
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			return false
		}
		if r.variety(st) != model.AtomicVariety {
			return false
		}
		if model.IsQNameOrNotation(st.Name()) {
			return true
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			base := st.Restriction.Base
			if (base.Namespace == model.XSDNamespace || base.Namespace == "") &&
				(base.Local == string(model.TypeNameQName) || base.Local == string(model.TypeNameNOTATION)) {
				return true
			}
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return false
	}
	return walk(typ)
}

func (r *typeResolver) isIntegerDerived(typ model.Type) bool {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) bool
	walk = func(current model.Type) bool {
		if current == nil {
			return false
		}
		if seen[current] {
			return false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return isIntegerTypeName(bt.Name().Local)
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			return false
		}
		if r.variety(st) != model.AtomicVariety {
			return false
		}
		if isIntegerTypeName(st.Name().Local) {
			return true
		}
		base := r.baseType(st)
		if base == nil {
			return false
		}
		return walk(base)
	}
	return walk(typ)
}

func (r *typeResolver) whitespaceMode(typ model.Type) runtime.WhitespaceMode {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) runtime.WhitespaceMode
	walk = func(current model.Type) runtime.WhitespaceMode {
		if current == nil {
			return runtime.WS_Preserve
		}
		if seen[current] {
			return runtime.WS_Preserve
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return wsmode.ToRuntime(bt.WhiteSpace())
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			return runtime.WS_Preserve
		}
		if st.WhiteSpaceExplicit() {
			return wsmode.ToRuntime(st.WhiteSpace())
		}
		if st.List != nil || st.Union != nil {
			return wsmode.ToRuntime(st.WhiteSpace())
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return wsmode.ToRuntime(st.WhiteSpace())
	}
	return walk(typ)
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
		if builtin := builtins.Get(builtins.TypeName(st.Name().Local)); builtin != nil {
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
