package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) lookupType(qname, referrer model.QName) (model.Type, error) {
	if bt := builtins.GetNS(qname.Namespace, qname.Local); bt != nil {
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

	switch t := typ.(type) {
	case *model.SimpleType:
		if err := r.resolveSimpleType(qname, t); err != nil {
			return nil, err
		}
	case *model.ComplexType:
		if err := r.resolveComplexType(qname, t); err != nil {
			return nil, err
		}
	}

	return typ, nil
}

type elementTypeOptions struct {
	simpleContext  string
	complexContext string
	allowResolving bool
}

func (r *Resolver) resolveElementType(elem *model.ElementDecl, elemName model.QName, opts elementTypeOptions) error {
	switch t := elem.Type.(type) {
	case *model.SimpleType:
		if model.IsPlaceholderSimpleType(t) {
			actualType, err := r.lookupType(t.QName, model.QName{})
			if err != nil {
				return fmt.Errorf(opts.simpleContext, elemName, err)
			}
			elem.Type = actualType
			return nil
		}
		if err := r.resolveSimpleType(t.QName, t); err != nil {
			return fmt.Errorf(opts.simpleContext, elemName, err)
		}
	case *model.ComplexType:
		if opts.allowResolving && !t.QName.IsZero() && r.detector.IsResolving(t.QName) {
			return nil
		}
		if err := r.resolveComplexType(t.QName, t); err != nil {
			return fmt.Errorf(opts.complexContext, elemName, err)
		}
	}
	return nil
}
