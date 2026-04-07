package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func (r *Resolver) lookupType(qname, referrer model.QName) (model.Type, error) {
	if bt := model.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return bt, nil
	}

	if qname == referrer {
		return nil, fmt.Errorf("circular type definition: %s references itself", qname)
	}

	typ, ok := r.schema.TypeDefs[qname]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTypeNotFound, qname)
	}

	if r.detector.IsResolving(qname) {
		if referrer.IsZero() {
			return typ, nil
		}
		return nil, fmt.Errorf("circular reference detected: %s", qname.String())
	}

	if err := r.resolveLookupType(qname, typ); err != nil {
		return nil, err
	}

	return typ, nil
}

func (r *Resolver) resolveLookupType(qname model.QName, typ model.Type) error {
	switch t := typ.(type) {
	case *model.SimpleType:
		return r.resolveSimpleType(qname, t)
	case *model.ComplexType:
		return r.resolveComplexType(qname, t)
	}
	return nil
}

type elementTypeOptions struct {
	simpleContext  string
	complexContext string
	allowResolving bool
}

func (r *Resolver) resolveElementType(elem *model.ElementDecl, elemName model.QName, opts elementTypeOptions) error {
	switch t := elem.Type.(type) {
	case *model.SimpleType:
		return r.resolveSimpleElementType(elem, elemName, t, opts)
	case *model.ComplexType:
		return r.resolveComplexElementType(elemName, t, opts)
	}
	return nil
}

func (r *Resolver) resolveSimpleElementType(elem *model.ElementDecl, elemName model.QName, st *model.SimpleType, opts elementTypeOptions) error {
	if model.IsPlaceholderSimpleType(st) {
		actualType, err := r.lookupType(st.QName, model.QName{})
		if err != nil {
			return fmt.Errorf(opts.simpleContext, elemName, err)
		}
		elem.Type = actualType
		return nil
	}
	if err := r.resolveSimpleType(st.QName, st); err != nil {
		return fmt.Errorf(opts.simpleContext, elemName, err)
	}
	return nil
}

func (r *Resolver) resolveComplexElementType(elemName model.QName, ct *model.ComplexType, opts elementTypeOptions) error {
	if opts.allowResolving && !ct.QName.IsZero() && r.detector.IsResolving(ct.QName) {
		return nil
	}
	if err := r.resolveComplexType(ct.QName, ct); err != nil {
		return fmt.Errorf(opts.complexContext, elemName, err)
	}
	return nil
}
