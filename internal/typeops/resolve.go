package typeops

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// TypeReferencePolicy controls how missing type references are handled.
type TypeReferencePolicy int

const (
	// TypeReferenceMustExist requires referenced types to resolve.
	TypeReferenceMustExist TypeReferencePolicy = iota
	// TypeReferenceAllowMissing allows unresolved placeholders to pass through.
	TypeReferenceAllowMissing
)

// ResolveTypeQName resolves a type QName against built-ins and schema types.
func ResolveTypeQName(schema *parser.Schema, qname types.QName, policy TypeReferencePolicy) (types.Type, error) {
	if qname.IsZero() {
		return nil, nil
	}
	if builtinType := types.GetBuiltinNS(qname.Namespace, qname.Local); builtinType != nil {
		return builtinType, nil
	}
	if schema != nil {
		if typeDef, ok := schema.TypeDefs[qname]; ok {
			return typeDef, nil
		}
	}
	if policy == TypeReferenceAllowMissing {
		return nil, nil
	}
	return nil, fmt.Errorf("type %s not found", qname)
}

// ResolveTypeReference resolves a type reference in schema validation contexts.
func ResolveTypeReference(schema *parser.Schema, typ types.Type, policy TypeReferencePolicy) types.Type {
	if typ == nil {
		return nil
	}
	if simpleType, ok := typ.(*types.SimpleType); ok && types.IsPlaceholderSimpleType(simpleType) {
		resolvedType, err := ResolveTypeQName(schema, simpleType.QName, policy)
		if err != nil {
			return nil
		}
		if resolvedType == nil && policy == TypeReferenceAllowMissing {
			return typ
		}
		return resolvedType
	}
	return typ
}

// ResolveSimpleTypeReferenceAllowMissing resolves a simple type QName when present.
func ResolveSimpleTypeReferenceAllowMissing(schema *parser.Schema, qname types.QName) types.Type {
	resolved, err := ResolveTypeQName(schema, qname, TypeReferenceAllowMissing)
	if err != nil {
		return nil
	}
	return resolved
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
	visited := make(map[*types.SimpleType]bool)
	var visit func(*types.SimpleType) []types.Type
	visit = func(current *types.SimpleType) []types.Type {
		if current == nil {
			return nil
		}
		if visited[current] {
			return nil
		}
		visited[current] = true
		defer delete(visited, current)

		if len(current.MemberTypes) > 0 {
			return current.MemberTypes
		}
		if current.Union != nil {
			memberTypes := make([]types.Type, 0, len(current.Union.MemberTypes)+len(current.Union.InlineTypes))
			for _, inline := range current.Union.InlineTypes {
				memberTypes = append(memberTypes, inline)
			}
			for _, memberQName := range current.Union.MemberTypes {
				if member := ResolveSimpleTypeReferenceAllowMissing(schema, memberQName); member != nil {
					memberTypes = append(memberTypes, member)
				}
			}
			return memberTypes
		}
		if current.Restriction != nil && !current.Restriction.Base.IsZero() {
			if baseST, ok := ResolveSimpleTypeReferenceAllowMissing(schema, current.Restriction.Base).(*types.SimpleType); ok {
				return visit(baseST)
			}
		}
		return nil
	}

	return visit(st)
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
			if base := ResolveSimpleTypeReferenceAllowMissing(schema, st.Restriction.Base); base != nil {
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
		return ResolveSimpleTypeReferenceAllowMissing(schema, st.List.ItemType)
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
	visited := make(map[*types.SimpleType]bool)
	var visit func(*types.SimpleType) bool
	visit = func(current *types.SimpleType) bool {
		if current == nil || current.Restriction == nil {
			return false
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		defer delete(visited, current)

		baseQName := current.Restriction.Base
		if IsIDOnlyType(baseQName) {
			return true
		}

		var baseType types.Type
		if current.ResolvedBase != nil {
			baseType = current.ResolvedBase
		} else if !baseQName.IsZero() {
			baseType = ResolveSimpleTypeReferenceAllowMissing(schema, baseQName)
		}

		switch typed := baseType.(type) {
		case *types.SimpleType:
			return visit(typed)
		case *types.BuiltinType:
			return IsIDOnlyType(typed.Name())
		default:
			return false
		}
	}

	return visit(st)
}
