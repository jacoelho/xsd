package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/resolveguard"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveSimpleType(qname types.QName, st *types.SimpleType) error {
	if qname.IsZero() {
		return r.anonymousTypeGuard.Resolve(st, func() error {
			return fmt.Errorf("circular anonymous type definition")
		}, func() error {
			return r.doResolveSimpleType(qname, st)
		})
	}
	return resolveguard.ResolveNamed[types.QName](r.detector, qname, func() error {
		return r.doResolveSimpleType(qname, st)
	})
}

func (r *Resolver) doResolveSimpleType(qname types.QName, st *types.SimpleType) error {
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
