package typeops

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ResolveSimpleTypeReference resolves a simple type QName against built-ins and schema types.
func ResolveSimpleTypeReference(schema *parser.Schema, qname types.QName) types.Type {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == types.XSDNamespace {
		if bt := types.GetBuiltin(types.TypeName(qname.Local)); bt != nil {
			return bt
		}
	}
	if schema == nil {
		return nil
	}
	if typ, ok := schema.TypeDefs[qname]; ok {
		return typ
	}
	return nil
}

// ResolveSimpleContentBaseTypeFromContent resolves the base type of a simpleContent definition.
func ResolveSimpleContentBaseTypeFromContent(schema *parser.Schema, sc *types.SimpleContent) types.Type {
	if sc == nil {
		return nil
	}
	var baseQName types.QName
	if sc.Extension != nil {
		baseQName = sc.Extension.Base
	} else if sc.Restriction != nil {
		baseQName = sc.Restriction.Base
	}
	if baseQName.IsZero() {
		return nil
	}
	if bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local); bt != nil {
		return bt
	}
	if schema == nil {
		return nil
	}
	if resolvedType, ok := schema.TypeDefs[baseQName]; ok {
		return resolvedType
	}
	return nil
}

// ResolveUnionMemberTypes returns flattened member types for union simple types.
func ResolveUnionMemberTypes(schema *parser.Schema, st *types.SimpleType) []types.Type {
	return resolveUnionMemberTypesVisited(schema, st, make(map[*types.SimpleType]bool))
}

func resolveUnionMemberTypesVisited(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool) []types.Type {
	if st == nil {
		return nil
	}
	if visited[st] {
		return nil
	}
	visited[st] = true
	defer delete(visited, st)

	if len(st.MemberTypes) > 0 {
		return st.MemberTypes
	}
	if st.Union != nil {
		memberTypes := make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
		for _, inline := range st.Union.InlineTypes {
			memberTypes = append(memberTypes, inline)
		}
		for _, memberQName := range st.Union.MemberTypes {
			if member := ResolveSimpleTypeReference(schema, memberQName); member != nil {
				memberTypes = append(memberTypes, member)
			}
		}
		return memberTypes
	}
	if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		if baseST, ok := ResolveSimpleTypeReference(schema, st.Restriction.Base).(*types.SimpleType); ok {
			return resolveUnionMemberTypesVisited(schema, baseST, visited)
		}
	}
	return nil
}

// ResolveListItemType returns the list item type for explicit or derived list simple types.
func ResolveListItemType(schema *parser.Schema, st *types.SimpleType) types.Type {
	if st == nil || st.List == nil {
		if st == nil {
			return nil
		}
		if itemType, ok := types.ListItemType(st); ok {
			return itemType
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			if base := ResolveSimpleTypeReference(schema, st.Restriction.Base); base != nil {
				if itemType, ok := types.ListItemType(base); ok {
					return itemType
				}
			}
		}
		return nil
	}
	if st.ItemType != nil {
		return st.ItemType
	}
	if st.List.InlineItemType != nil {
		return st.List.InlineItemType
	}
	if !st.List.ItemType.IsZero() {
		return ResolveSimpleTypeReference(schema, st.List.ItemType)
	}
	if itemType, ok := types.ListItemType(st); ok {
		return itemType
	}
	return nil
}

// IsIDOnlyType reports whether the QName identifies xs:ID.
func IsIDOnlyType(qname types.QName) bool {
	return qname.Namespace == types.XSDNamespace && qname.Local == string(types.TypeNameID)
}

// IsIDOnlyDerivedType reports whether a simple type derives from xs:ID.
func IsIDOnlyDerivedType(schema *parser.Schema, st *types.SimpleType) bool {
	return isIDOnlyDerivedTypeVisited(schema, st, make(map[*types.SimpleType]bool))
}

func isIDOnlyDerivedTypeVisited(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool) bool {
	if st == nil || st.Restriction == nil {
		return false
	}
	if visited[st] {
		return false
	}
	visited[st] = true
	defer delete(visited, st)

	baseQName := st.Restriction.Base
	if IsIDOnlyType(baseQName) {
		return true
	}

	var baseType types.Type
	if st.ResolvedBase != nil {
		baseType = st.ResolvedBase
	} else if !baseQName.IsZero() {
		baseType = ResolveSimpleTypeReference(schema, baseQName)
	}

	switch typed := baseType.(type) {
	case *types.SimpleType:
		return isIDOnlyDerivedTypeVisited(schema, typed, visited)
	case *types.BuiltinType:
		return IsIDOnlyType(typed.Name())
	default:
		return false
	}
}
