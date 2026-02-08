package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// resolveSimpleTypeRestrictionBase resolves the base type for a simple type restriction
func resolveSimpleTypeRestrictionBase(schema *parser.Schema, st *types.SimpleType, restriction *types.Restriction) types.Type {
	if st != nil && st.ResolvedBase != nil {
		return st.ResolvedBase
	}
	if restriction != nil && restriction.SimpleType != nil {
		return restriction.SimpleType
	}
	if restriction == nil || restriction.Base.IsZero() {
		return nil
	}
	return typeops.ResolveSimpleTypeReferenceAllowMissing(schema, restriction.Base)
}

// resolveSimpleContentBaseType resolves the base type for a simpleContent restriction
func resolveSimpleContentBaseType(schema *parser.Schema, baseQName types.QName) (types.Type, types.QName) {
	return resolveSimpleContentBaseTypeVisited(schema, baseQName, make(map[types.QName]bool))
}

func resolveSimpleContentBaseTypeVisited(schema *parser.Schema, baseQName types.QName, visited map[types.QName]bool) (types.Type, types.QName) {
	if baseQName.IsZero() {
		return nil, baseQName
	}
	if visited[baseQName] {
		return nil, baseQName
	}
	visited[baseQName] = true

	if baseQName.Namespace == types.XSDNamespace {
		if bt := types.GetBuiltin(types.TypeName(baseQName.Local)); bt != nil {
			return bt, baseQName
		}
	}

	baseType, ok := lookupTypeDef(schema, baseQName)
	if !ok || baseType == nil {
		return nil, baseQName
	}

	ct, ok := baseType.(*types.ComplexType)
	if !ok {
		return baseType, baseQName
	}
	sc, ok := ct.Content().(*types.SimpleContent)
	if !ok {
		return baseType, baseQName
	}

	nextQName := sc.BaseTypeQName()
	if nextQName.IsZero() {
		return nil, baseQName
	}

	resolved, resolvedQName := resolveSimpleContentBaseTypeVisited(schema, nextQName, visited)
	if resolved != nil {
		return resolved, resolvedQName
	}
	return nil, nextQName
}
