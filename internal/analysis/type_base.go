package analysis

import (
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/types"
)

func baseTypeFor(schema *parser.Schema, typ types.Type) (types.Type, types.DerivationMethod, error) {
	switch typed := typ.(type) {
	case *types.SimpleType:
		return baseTypeForSimpleType(schema, typed)
	case *types.ComplexType:
		return baseTypeForComplexType(schema, typed)
	default:
		return nil, 0, nil
	}
}

func baseTypeForSimpleType(schema *parser.Schema, st *types.SimpleType) (types.Type, types.DerivationMethod, error) {
	if st == nil {
		return nil, 0, nil
	}
	if st.List != nil {
		return builtins.Get(builtins.TypeNameAnySimpleType), types.DerivationList, nil
	}
	if st.Union != nil {
		return builtins.Get(builtins.TypeNameAnySimpleType), types.DerivationUnion, nil
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			return st.Restriction.SimpleType, types.DerivationRestriction, nil
		}
		if !st.Restriction.Base.IsZero() {
			base, err := typeresolve.ResolveTypeQName(schema, st.Restriction.Base, typeresolve.TypeReferenceMustExist)
			if err != nil {
				return nil, 0, err
			}
			return base, types.DerivationRestriction, nil
		}
	}
	if st.ResolvedBase != nil {
		return st.ResolvedBase, types.DerivationRestriction, nil
	}
	return builtins.Get(builtins.TypeNameAnySimpleType), types.DerivationRestriction, nil
}

func baseTypeForComplexType(schema *parser.Schema, ct *types.ComplexType) (types.Type, types.DerivationMethod, error) {
	if ct == nil {
		return nil, 0, nil
	}
	baseQName := types.QName{}
	if content := ct.Content(); content != nil {
		baseQName = content.BaseTypeQName()
	}
	if baseQName.IsZero() {
		if ct.QName.Namespace == types.XSDNamespace && ct.QName.Local == "anyType" {
			return nil, 0, nil
		}
		return builtins.Get(builtins.TypeNameAnyType), types.DerivationRestriction, nil
	}
	method := ct.DerivationMethod
	if method == 0 {
		method = types.DerivationRestriction
	}
	base, err := typeresolve.ResolveTypeQName(schema, baseQName, typeresolve.TypeReferenceMustExist)
	if err != nil {
		return nil, 0, err
	}
	return base, method, nil
}
