package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typegraph"
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
	visited := make(map[types.QName]bool)
	var visit func(qname types.QName) (types.Type, types.QName)
	visit = func(qname types.QName) (types.Type, types.QName) {
		if qname.IsZero() {
			return nil, qname
		}
		if visited[qname] {
			return nil, qname
		}
		visited[qname] = true

		if qname.Namespace == types.XSDNamespace {
			if bt := types.GetBuiltin(types.TypeName(qname.Local)); bt != nil {
				return bt, qname
			}
		}

		baseType, ok := typegraph.LookupType(schema, qname)
		if !ok || baseType == nil {
			return nil, qname
		}

		ct, ok := baseType.(*types.ComplexType)
		if !ok {
			return baseType, qname
		}
		sc, ok := ct.Content().(*types.SimpleContent)
		if !ok {
			return baseType, qname
		}

		nextQName := sc.BaseTypeQName()
		if nextQName.IsZero() {
			return nil, qname
		}

		resolved, resolvedQName := visit(nextQName)
		if resolved != nil {
			return resolved, resolvedQName
		}
		return nil, nextQName
	}
	return visit(baseQName)
}
