package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

type typeResolver struct {
	schema *parser.Schema
}

func newTypeResolver(schema *parser.Schema) *typeResolver {
	return &typeResolver{schema: schema}
}

func (r *typeResolver) baseType(st *types.SimpleType) types.Type {
	if st == nil {
		return nil
	}
	if st.ResolvedBase != nil {
		if isAnySimpleType(st.ResolvedBase) {
			return nil
		}
		return st.ResolvedBase
	}
	if st.Restriction == nil {
		return nil
	}
	if st.Restriction.SimpleType != nil {
		if isAnySimpleType(st.Restriction.SimpleType) {
			return nil
		}
		return st.Restriction.SimpleType
	}
	if st.Restriction.Base.IsZero() {
		return nil
	}
	base := r.resolveQName(st.Restriction.Base)
	if isAnySimpleType(base) {
		return nil
	}
	return base
}

func (r *typeResolver) listItemType(st *types.SimpleType) (types.Type, bool) {
	return r.listItemTypeFromType(st)
}

func (r *typeResolver) listItemTypeFromType(typ types.Type) (types.Type, bool) {
	return r.listItemTypeFromTypeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) listItemTypeFromTypeSeen(typ types.Type, seen map[types.Type]bool) (types.Type, bool) {
	if typ == nil {
		return nil, false
	}
	if seen[typ] {
		return nil, false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		if itemName, ok := types.BuiltinListItemTypeName(bt.Name().Local); ok {
			if item := types.GetBuiltin(itemName); item != nil {
				return item, true
			}
		}
		return nil, false
	}

	st, ok := types.AsSimpleType(typ)
	if !ok {
		return nil, false
	}
	if r.variety(st) != types.ListVariety {
		return nil, false
	}
	if st.ItemType != nil {
		return st.ItemType, true
	}
	if st.List != nil {
		if st.List.InlineItemType != nil {
			return st.List.InlineItemType, true
		}
		if !st.List.ItemType.IsZero() {
			if item := r.resolveQName(st.List.ItemType); item != nil {
				return item, true
			}
		}
	}
	if base := r.baseType(st); base != nil {
		return r.listItemTypeFromTypeSeen(base, seen)
	}
	return nil, false
}

func (r *typeResolver) unionMemberTypes(st *types.SimpleType) []types.Type {
	return r.unionMemberTypesFromType(st)
}

func (r *typeResolver) unionMemberTypesFromType(typ types.Type) []types.Type {
	return r.unionMemberTypesFromTypeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) unionMemberTypesFromTypeSeen(typ types.Type, seen map[types.Type]bool) []types.Type {
	if typ == nil {
		return nil
	}
	if seen[typ] {
		return nil
	}
	seen[typ] = true
	defer delete(seen, typ)

	st, ok := types.AsSimpleType(typ)
	if !ok {
		return nil
	}
	if r.variety(st) != types.UnionVariety {
		return nil
	}
	if len(st.MemberTypes) > 0 {
		return st.MemberTypes
	}
	if st.Union != nil {
		members := make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
		for _, qname := range st.Union.MemberTypes {
			if member := r.resolveQName(qname); member != nil {
				members = append(members, member)
			}
		}
		for _, inline := range st.Union.InlineTypes {
			members = append(members, inline)
		}
		if len(members) > 0 {
			return members
		}
	}
	if base := r.baseType(st); base != nil {
		return r.unionMemberTypesFromTypeSeen(base, seen)
	}
	return nil
}

func (r *typeResolver) variety(st *types.SimpleType) types.SimpleTypeVariety {
	if st == nil {
		return types.AtomicVariety
	}
	if st.List != nil {
		return types.ListVariety
	}
	if st.Union != nil {
		return types.UnionVariety
	}
	if st.ResolvedBase != nil {
		if baseST, ok := types.AsSimpleType(st.ResolvedBase); ok {
			return r.variety(baseST)
		}
		if bt := builtinForType(st.ResolvedBase); bt != nil && isBuiltinListName(bt.Name().Local) {
			return types.ListVariety
		}
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			if baseST, ok := types.AsSimpleType(st.Restriction.SimpleType); ok {
				return r.variety(baseST)
			}
			if bt := builtinForType(st.Restriction.SimpleType); bt != nil && isBuiltinListName(bt.Name().Local) {
				return types.ListVariety
			}
		}
		if !st.Restriction.Base.IsZero() {
			if base := r.resolveQName(st.Restriction.Base); base != nil {
				if baseST, ok := types.AsSimpleType(base); ok {
					return r.variety(baseST)
				}
				if bt := builtinForType(base); bt != nil && isBuiltinListName(bt.Name().Local) {
					return types.ListVariety
				}
			}
		}
	}
	if st.IsBuiltin() && isBuiltinListName(st.Name().Local) {
		return types.ListVariety
	}
	return types.AtomicVariety
}

func (r *typeResolver) varietyForType(typ types.Type) types.SimpleTypeVariety {
	if typ == nil {
		return types.AtomicVariety
	}
	if bt := builtinForType(typ); bt != nil {
		if isBuiltinListName(bt.Name().Local) {
			return types.ListVariety
		}
		return types.AtomicVariety
	}
	if st, ok := types.AsSimpleType(typ); ok {
		return r.variety(st)
	}
	return types.AtomicVariety
}

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

func (r *typeResolver) isIntegerDerived(typ types.Type) bool {
	return r.isIntegerDerivedSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) isIntegerDerivedSeen(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	if seen[typ] {
		return false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return isIntegerTypeName(bt.Name().Local)
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return false
	}
	if r.variety(st) != types.AtomicVariety {
		return false
	}
	if isIntegerTypeName(st.Name().Local) {
		return true
	}
	base := r.baseType(st)
	if base == nil {
		return false
	}
	return r.isIntegerDerivedSeen(base, seen)
}

func (r *typeResolver) isQNameOrNotation(typ types.Type) bool {
	return r.isQNameOrNotationSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) isQNameOrNotationSeen(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	if seen[typ] {
		return false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return bt.IsQNameOrNotationType()
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return false
	}
	if r.variety(st) != types.AtomicVariety {
		return false
	}
	if types.IsQNameOrNotation(st.Name()) {
		return true
	}
	if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		base := st.Restriction.Base
		if (base.Namespace == types.XSDNamespace || base.Namespace.IsEmpty()) &&
			(base.Local == string(types.TypeNameQName) || base.Local == string(types.TypeNameNOTATION)) {
			return true
		}
	}
	if base := r.baseType(st); base != nil {
		return r.isQNameOrNotationSeen(base, seen)
	}
	return false
}

func (r *typeResolver) whitespaceMode(typ types.Type) runtime.WhitespaceMode {
	return r.whitespaceModeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) whitespaceModeSeen(typ types.Type, seen map[types.Type]bool) runtime.WhitespaceMode {
	if typ == nil {
		return runtime.WS_Preserve
	}
	if seen[typ] {
		return runtime.WS_Preserve
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return toRuntimeWhitespaceMode(bt.WhiteSpace())
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return runtime.WS_Preserve
	}
	if st.WhiteSpaceExplicit() {
		return toRuntimeWhitespaceMode(st.WhiteSpace())
	}
	if st.List != nil || st.Union != nil {
		return toRuntimeWhitespaceMode(st.WhiteSpace())
	}
	if base := r.baseType(st); base != nil {
		return r.whitespaceModeSeen(base, seen)
	}
	return toRuntimeWhitespaceMode(st.WhiteSpace())
}

func (r *typeResolver) isListType(typ types.Type) bool {
	return r.varietyForType(typ) == types.ListVariety
}

func (r *typeResolver) resolveQName(name types.QName) types.Type {
	if name.IsZero() {
		return nil
	}
	if builtin := types.GetBuiltinNS(name.Namespace, name.Local); builtin != nil {
		return builtin
	}
	if r.schema == nil {
		return nil
	}
	if def, ok := r.schema.TypeDefs[name]; ok {
		return def
	}
	return nil
}

func (r *typeResolver) builtinNameForType(typ types.Type) (types.TypeName, bool) {
	return r.builtinNameForTypeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) builtinNameForTypeSeen(typ types.Type, seen map[types.Type]bool) (types.TypeName, bool) {
	if typ == nil {
		return "", false
	}
	if seen[typ] {
		return "", false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return types.TypeName(bt.Name().Local), true
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return "", false
	}
	if base := r.baseType(st); base != nil {
		return r.builtinNameForTypeSeen(base, seen)
	}
	return "", false
}

func builtinForType(typ types.Type) *types.BuiltinType {
	if typ == nil {
		return nil
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		return bt
	}
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		return types.GetBuiltin(types.TypeName(st.Name().Local))
	}
	return nil
}

func isBuiltinListName(name string) bool {
	return types.IsBuiltinListTypeName(name)
}

func isIntegerTypeName(name string) bool {
	switch name {
	case "integer", "long", "int", "short", "byte",
		"unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte",
		"nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		return true
	default:
		return false
	}
}

func isAnySimpleType(typ types.Type) bool {
	bt := builtinForType(typ)
	if bt == nil {
		return false
	}
	return bt.Name().Local == string(types.TypeNameAnySimpleType)
}
