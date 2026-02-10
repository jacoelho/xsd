package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func (r *Resolver) resolveSimpleType(qname model.QName, st *model.SimpleType) error {
	if qname.IsZero() {
		if r.resolvedPtrs[st] {
			return nil
		}
		if r.resolvingPtrs[st] {
			return fmt.Errorf("circular anonymous type definition")
		}
		r.resolvingPtrs[st] = true
		defer func() {
			delete(r.resolvingPtrs, st)
			r.resolvedPtrs[st] = true
		}()
		return r.doResolveSimpleType(qname, st)
	}

	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
		return r.doResolveSimpleType(qname, st)
	})
}

func (r *Resolver) doResolveSimpleType(qname model.QName, st *model.SimpleType) error {
	if err := r.resolveSimpleTypeRestriction(qname, st); err != nil {
		return err
	}
	if err := r.resolveSimpleTypeList(qname, st); err != nil {
		return err
	}
	if err := r.resolveSimpleTypeUnion(qname, st); err != nil {
		return err
	}
	return nil
}
