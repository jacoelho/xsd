package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/identity"
)

func collectIdentityAttrs(rt *runtime.Schema, startAttrs []attrs.Start, applied []attrs.Applied, intern func(ns, local []byte) identity.AttrNameID) []identity.Attr {
	if len(startAttrs) == 0 && len(applied) == 0 {
		return nil
	}
	rawAttrs := make([]identity.RawAttr, 0, len(startAttrs))
	for _, attr := range startAttrs {
		rawAttrs = append(rawAttrs, identity.RawAttr{
			NSBytes:  attr.NSBytes,
			Local:    attr.Local,
			KeyBytes: attr.KeyBytes,
			Sym:      attr.Sym,
			NS:       attr.NS,
			KeyKind:  attr.KeyKind,
		})
	}
	appliedAttrs := make([]identity.AppliedAttr, 0, len(applied))
	for _, ap := range applied {
		appliedAttrs = append(appliedAttrs, identity.AppliedAttr{
			Name:     ap.Name,
			KeyBytes: ap.KeyBytes,
			KeyKind:  ap.KeyKind,
		})
	}
	return identity.CollectAttrs(rt, rawAttrs, appliedAttrs, intern)
}
