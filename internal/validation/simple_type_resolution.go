package validation

import (
	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// resolveSimpleTypeRestrictionBase resolves the base type for a simple type restriction
func resolveSimpleTypeRestrictionBase(schema *schema.Schema, st *types.SimpleType, restriction *types.Restriction) types.Type {
	if st != nil && st.ResolvedBase != nil {
		return st.ResolvedBase
	}
	if restriction == nil || restriction.Base.IsZero() {
		return nil
	}
	return resolveSimpleTypeReference(schema, restriction.Base)
}

// resolveSimpleTypeReference resolves a simple type reference by QName
func resolveSimpleTypeReference(schema *schema.Schema, qname types.QName) types.Type {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == types.XSDNamespace {
		if bt := types.GetBuiltin(types.TypeName(qname.Local)); bt != nil {
			return bt
		}
	}
	if schema != nil {
		if typ, ok := schema.TypeDefs[qname]; ok {
			return typ
		}
	}
	return nil
}

// resolveSimpleContentBaseType resolves the base type for a simpleContent restriction
func resolveSimpleContentBaseType(schema *schema.Schema, baseQName types.QName) (types.Type, types.QName) {
	if baseQName.IsZero() {
		return nil, baseQName
	}

	if baseQName.Namespace == types.XSDNamespace {
		if bt := types.GetBuiltin(types.TypeName(baseQName.Local)); bt != nil {
			return bt, baseQName
		}
	}

	baseType, ok := schema.TypeDefs[baseQName]
	if !ok || baseType == nil {
		return nil, baseQName
	}

	if ct, ok := baseType.(*types.ComplexType); ok {
		if _, ok := ct.Content().(*types.SimpleContent); ok {
			base := types.ResolveSimpleContentBaseType(ct.BaseType())
			if base != nil {
				return base, base.Name()
			}
		}
		return baseType, baseQName
	}

	return baseType, baseQName
}
