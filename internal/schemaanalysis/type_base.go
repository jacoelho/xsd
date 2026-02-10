package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
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
			base, err := resolveTypeQName(schema, st.Restriction.Base)
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
	base, err := resolveTypeQName(schema, baseQName)
	if err != nil {
		return nil, 0, err
	}
	return base, method, nil
}

func resolveTypeQName(schema *parser.Schema, qname model.QName) (model.Type, error) {
	if qname.IsZero() {
		return nil, nil
	}
	if builtin := builtins.GetNS(qname.Namespace, qname.Local); builtin != nil {
		return builtin, nil
	}
	if schema == nil {
		return nil, fmt.Errorf("type %s not found", qname)
	}
	if resolved := schema.TypeDefs[qname]; resolved != nil {
		return resolved, nil
	}
	return nil, fmt.Errorf("type %s not found", qname)
}
