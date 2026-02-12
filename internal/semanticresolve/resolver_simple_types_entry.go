package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/resolveguard"
	model "github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveSimpleType(qname model.QName, st *model.SimpleType) error {
	if qname.IsZero() {
		return r.anonymousTypeGuard.Resolve(st, func() error {
			return fmt.Errorf("circular anonymous type definition")
		}, func() error {
			return r.doResolveSimpleType(qname, st)
		})
	}
	return resolveguard.ResolveNamed[model.QName](r.detector, qname, func() error {
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
