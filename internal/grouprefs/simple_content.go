package grouprefs

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

// TypeLookupFunc resolves type QNames for simple-content base lookups.
type TypeLookupFunc func(name model.QName) model.Type

// SimpleContentTextTypeOptions configures simple-content text-type resolution.
type SimpleContentTextTypeOptions struct {
	ResolveQName TypeLookupFunc
	Cache        map[*model.ComplexType]model.Type
}

// ResolveSimpleContentTextType resolves the effective text type of complex simpleContent.
func ResolveSimpleContentTextType(ct *model.ComplexType, opts SimpleContentTextTypeOptions) (model.Type, error) {
	if ct == nil {
		return nil, nil
	}
	if cached, ok := opts.Cache[ct]; ok {
		return cached, nil
	}
	seen := make(map[*model.ComplexType]bool)
	return resolveSimpleContentTextType(ct, opts, seen)
}

func resolveSimpleContentTextType(
	ct *model.ComplexType,
	opts SimpleContentTextTypeOptions,
	seen map[*model.ComplexType]bool,
) (model.Type, error) {
	if ct == nil {
		return nil, nil
	}
	if cached, ok := opts.Cache[ct]; ok {
		return cached, nil
	}
	if seen[ct] {
		return nil, fmt.Errorf("simpleContent cycle detected")
	}
	seen[ct] = true
	defer delete(seen, ct)

	sc, ok := ct.Content().(*model.SimpleContent)
	if !ok {
		return nil, nil
	}
	baseType, err := resolveSimpleContentBaseType(ct, sc, opts, seen)
	if err != nil {
		return nil, err
	}

	var result model.Type
	switch {
	case sc.Extension != nil:
		result = baseType
	case sc.Restriction != nil:
		st := &model.SimpleType{
			Restriction:  sc.Restriction,
			ResolvedBase: baseType,
		}
		if sc.Restriction.SimpleType != nil && sc.Restriction.SimpleType.WhiteSpaceExplicit() {
			st.SetWhiteSpaceExplicit(sc.Restriction.SimpleType.WhiteSpace())
		} else if baseType != nil {
			st.SetWhiteSpace(baseType.WhiteSpace())
		}
		result = st
	default:
		result = baseType
	}
	if result != nil && opts.Cache != nil {
		opts.Cache[ct] = result
	}
	return result, nil
}

func resolveSimpleContentBaseType(
	ct *model.ComplexType,
	sc *model.SimpleContent,
	opts SimpleContentTextTypeOptions,
	seen map[*model.ComplexType]bool,
) (model.Type, error) {
	base := ct.ResolvedBase
	if base == nil {
		qname := sc.BaseTypeQName()
		if !qname.IsZero() && opts.ResolveQName != nil {
			base = opts.ResolveQName(qname)
		}
	}
	if base == nil {
		return nil, fmt.Errorf("simpleContent base missing")
	}
	switch typed := base.(type) {
	case *model.SimpleType, *model.BuiltinType:
		return typed, nil
	case *model.ComplexType:
		return resolveSimpleContentTextType(typed, opts, seen)
	default:
		return nil, fmt.Errorf("simpleContent base is not simple")
	}
}
