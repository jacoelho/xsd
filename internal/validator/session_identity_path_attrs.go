package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func collectIdentityAttrs(rt *runtime.Schema, attrs []StartAttr, applied []AttrApplied) []rtIdentityAttr {
	if len(attrs) == 0 && len(applied) == 0 {
		return nil
	}
	out := make([]rtIdentityAttr, 0, len(attrs)+len(applied))
	for _, attr := range attrs {
		local := attr.Local
		if len(local) == 0 && attr.Sym != 0 {
			local = rt.Symbols.LocalBytes(attr.Sym)
		}
		nsBytes := attr.NSBytes
		if len(nsBytes) == 0 && attr.NS != 0 {
			nsBytes = rt.Namespaces.Bytes(attr.NS)
		}
		out = append(out, rtIdentityAttr{
			sym:      attr.Sym,
			ns:       attr.NS,
			nsBytes:  nsBytes,
			local:    local,
			keyKind:  attr.KeyKind,
			keyBytes: attr.KeyBytes,
		})
	}
	for _, ap := range applied {
		if ap.Name == 0 {
			continue
		}
		nsID := runtime.NamespaceID(0)
		if int(ap.Name) < len(rt.Symbols.NS) {
			nsID = rt.Symbols.NS[ap.Name]
		}
		out = append(out, rtIdentityAttr{
			sym:      ap.Name,
			ns:       nsID,
			nsBytes:  rt.Namespaces.Bytes(nsID),
			local:    rt.Symbols.LocalBytes(ap.Name),
			keyKind:  ap.KeyKind,
			keyBytes: ap.KeyBytes,
		})
	}
	return out
}

func isXMLNSAttr(attr *rtIdentityAttr, rt *runtime.Schema) bool {
	if rt == nil || attr == nil {
		return false
	}
	if attr.ns != 0 {
		nsBytes := rt.Namespaces.Bytes(attr.ns)
		return bytes.Equal(nsBytes, []byte(xsdxml.XMLNSNamespace))
	}
	return bytes.Equal(attr.nsBytes, []byte(xsdxml.XMLNSNamespace))
}

func attrNamespaceMatches(attr *rtIdentityAttr, ns runtime.NamespaceID, rt *runtime.Schema) bool {
	if attr == nil {
		return false
	}
	if attr.ns != 0 {
		return attr.ns == ns
	}
	if rt == nil {
		return false
	}
	return bytes.Equal(attr.nsBytes, rt.Namespaces.Bytes(ns))
}

func attrNameMatches(attr *rtIdentityAttr, op runtime.PathOp, rt *runtime.Schema) bool {
	if attr == nil {
		return false
	}
	if attr.sym != 0 {
		return attr.sym == op.Sym
	}
	if rt == nil {
		return false
	}
	targetLocal := rt.Symbols.LocalBytes(op.Sym)
	if !bytes.Equal(attr.local, targetLocal) {
		return false
	}
	return attrNamespaceMatches(attr, op.NS, rt)
}

func makeAttrKey(nsBytes, local []byte) string {
	if len(nsBytes) == 0 && len(local) == 0 {
		return ""
	}
	key := make([]byte, 0, len(nsBytes)+1+len(local))
	key = append(key, nsBytes...)
	key = append(key, 0)
	key = append(key, local...)
	return string(key)
}
