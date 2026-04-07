package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func collectIdentityAttrs(rt *runtime.Schema, startAttrs []Start, applied []Applied, intern func(ns, local []byte) AttrNameID) []Attr {
	if len(startAttrs) == 0 && len(applied) == 0 {
		return nil
	}
	rawAttrs := make([]RawAttr, 0, len(startAttrs))
	for _, attr := range startAttrs {
		rawAttrs = append(rawAttrs, RawAttr{
			NSBytes:  attr.NSBytes,
			Local:    attr.Local,
			KeyBytes: attr.KeyBytes,
			Sym:      attr.Sym,
			NS:       attr.NS,
			KeyKind:  attr.KeyKind,
		})
	}
	appliedAttrs := make([]AppliedAttr, 0, len(applied))
	for _, ap := range applied {
		appliedAttrs = append(appliedAttrs, AppliedAttr{
			Name:     ap.Name,
			KeyBytes: ap.KeyBytes,
			KeyKind:  ap.KeyKind,
		})
	}
	return CollectAttrs(rt, rawAttrs, appliedAttrs, intern)
}
