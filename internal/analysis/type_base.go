package analysis

import (
	"github.com/jacoelho/xsd/internal/builtins"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	model "github.com/jacoelho/xsd/internal/types"
)

func baseTypeFor(schema *parser.Schema, typ model.Type) (model.Type, model.DerivationMethod, error) {
	switch typed := typ.(type) {
	case *model.SimpleType:
		return baseTypeForSimpleType(schema, typed)
	case *model.ComplexType:
		return baseTypeForComplexType(schema, typed)
	default:
		return nil, 0, nil
	}
}

func baseTypeForSimpleType(schema *parser.Schema, st *model.SimpleType) (model.Type, model.DerivationMethod, error) {
	if st == nil {
		return nil, 0, nil
	}
	if st.List != nil {
		return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationList, nil
	}
	if st.Union != nil {
		return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationUnion, nil
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			return st.Restriction.SimpleType, model.DerivationRestriction, nil
		}
		if !st.Restriction.Base.IsZero() {
			base, err := typeresolve.ResolveTypeQName(schema, st.Restriction.Base, typeresolve.TypeReferenceMustExist)
			if err != nil {
				return nil, 0, err
			}
			return base, model.DerivationRestriction, nil
		}
	}
	if st.ResolvedBase != nil {
		return st.ResolvedBase, model.DerivationRestriction, nil
	}
	return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationRestriction, nil
}

func baseTypeForComplexType(schema *parser.Schema, ct *model.ComplexType) (model.Type, model.DerivationMethod, error) {
	if ct == nil {
		return nil, 0, nil
	}
	baseQName := model.QName{}
	if content := ct.Content(); content != nil {
		baseQName = content.BaseTypeQName()
	}
	if baseQName.IsZero() {
		if ct.QName.Namespace == model.XSDNamespace && ct.QName.Local == "anyType" {
			return nil, 0, nil
		}
		return builtins.Get(builtins.TypeNameAnyType), model.DerivationRestriction, nil
	}
	method := ct.DerivationMethod
	if method == 0 {
		method = model.DerivationRestriction
	}
	base, err := typeresolve.ResolveTypeQName(schema, baseQName, typeresolve.TypeReferenceMustExist)
	if err != nil {
		return nil, 0, err
	}
	return base, method, nil
}
