package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// resolveSimpleTypeRestrictionBase resolves the base type for a simple type restriction
func resolveSimpleTypeRestrictionBase(schema *parser.Schema, st *model.SimpleType, restriction *model.Restriction) model.Type {
	if st != nil && st.ResolvedBase != nil {
		return st.ResolvedBase
	}
	if restriction != nil && restriction.SimpleType != nil {
		return restriction.SimpleType
	}
	if restriction == nil || restriction.Base.IsZero() {
		return nil
	}
	return typeresolve.ResolveSimpleTypeReferenceAllowMissing(schema, restriction.Base)
}

// resolveSimpleContentBaseType resolves the base type for a simpleContent restriction
func resolveSimpleContentBaseType(schema *parser.Schema, baseQName model.QName) (model.Type, model.QName) {
	visited := make(map[model.QName]bool)
	var visit func(qname model.QName) (model.Type, model.QName)
	visit = func(qname model.QName) (model.Type, model.QName) {
		if qname.IsZero() {
			return nil, qname
		}
		if visited[qname] {
			return nil, qname
		}
		visited[qname] = true

		if qname.Namespace == model.XSDNamespace {
			if bt := builtins.Get(model.TypeName(qname.Local)); bt != nil {
				return bt, qname
			}
		}

		baseType, ok := typechain.LookupType(schema, qname)
		if !ok || baseType == nil {
			return nil, qname
		}

		ct, ok := baseType.(*model.ComplexType)
		if !ok {
			return baseType, qname
		}
		sc, ok := ct.Content().(*model.SimpleContent)
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
