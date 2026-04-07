package validator

import (
	"bytes"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlnames"
)

// AttrNameID identifies one interned attribute name within a validation session.
type AttrNameID uint32

// AttrName stores one session-local attribute name.
type AttrName struct {
	NS    []byte
	Local []byte
}

// AttrNames owns the session-local attribute-name interner.
type AttrNames struct {
	Buckets map[uint64][]AttrNameID
	Names   []AttrName
}

// Intern returns a stable session-local ID for the attribute name.
func (s *AttrNames) Intern(hash uint64, ns, local []byte) AttrNameID {
	if s == nil {
		return 0
	}
	if s.Buckets == nil {
		s.Buckets = make(map[uint64][]AttrNameID)
	}
	bucket := s.Buckets[hash]
	for _, id := range bucket {
		if id == 0 {
			continue
		}
		entry := s.Names[int(id)-1]
		if bytes.Equal(entry.NS, ns) && bytes.Equal(entry.Local, local) {
			return id
		}
	}
	id := AttrNameID(len(s.Names) + 1)
	s.Names = append(s.Names, AttrName{
		NS:    slices.Clone(ns),
		Local: slices.Clone(local),
	})
	s.Buckets[hash] = append(bucket, id)
	return id
}

// Reset clears the interner while dropping oversized backing storage.
func (s *AttrNames) Reset(entryLimit int) {
	if s == nil {
		return
	}
	if cap(s.Names) > entryLimit {
		s.Names = nil
	} else {
		s.Names = s.Names[:0]
	}
	if s.Buckets == nil {
		return
	}
	if len(s.Buckets) > entryLimit {
		s.Buckets = nil
		return
	}
	clear(s.Buckets)
}

// Attr is the normalized identity-constraint view of one attribute.
type Attr struct {
	NSBytes  []byte
	Local    []byte
	KeyBytes []byte
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	KeyKind  runtime.ValueKind
	NameID   AttrNameID
}

// RawAttr is the validator runtime view of one explicitly present attribute.
type RawAttr struct {
	NSBytes  []byte
	Local    []byte
	KeyBytes []byte
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	KeyKind  runtime.ValueKind
}

// AppliedAttr is the validator runtime view of one defaulted or fixed attribute.
type AppliedAttr struct {
	KeyBytes []byte
	Name     runtime.SymbolID
	KeyKind  runtime.ValueKind
}

// CollectAttrs normalizes explicit and applied attributes for identity processing.
func CollectAttrs(rt *runtime.Schema, attrs []RawAttr, applied []AppliedAttr, intern func(ns, local []byte) AttrNameID) []Attr {
	if len(attrs) == 0 && len(applied) == 0 {
		return nil
	}
	out := make([]Attr, 0, len(attrs)+len(applied))
	for _, attr := range attrs {
		local := attr.Local
		if len(local) == 0 && attr.Sym != 0 {
			local = rt.Symbols.LocalBytes(attr.Sym)
		}
		nsBytes := attr.NSBytes
		if len(nsBytes) == 0 && attr.NS != 0 {
			nsBytes = rt.Namespaces.Bytes(attr.NS)
		}
		nameID := AttrNameID(0)
		if attr.Sym == 0 && intern != nil {
			nameID = intern(nsBytes, local)
		}
		out = append(out, Attr{
			Sym:      attr.Sym,
			NS:       attr.NS,
			NSBytes:  nsBytes,
			Local:    local,
			KeyKind:  attr.KeyKind,
			KeyBytes: attr.KeyBytes,
			NameID:   nameID,
		})
	}
	for _, attr := range applied {
		if attr.Name == 0 {
			continue
		}
		nsID := runtime.NamespaceID(0)
		if int(attr.Name) < len(rt.Symbols.NS) {
			nsID = rt.Symbols.NS[attr.Name]
		}
		out = append(out, Attr{
			Sym:      attr.Name,
			NS:       nsID,
			NSBytes:  rt.Namespaces.Bytes(nsID),
			Local:    rt.Symbols.LocalBytes(attr.Name),
			KeyKind:  attr.KeyKind,
			KeyBytes: attr.KeyBytes,
		})
	}
	return out
}

// IsXMLNSAttr reports whether the attribute is an xmlns declaration.
func IsXMLNSAttr(attr *Attr, rt *runtime.Schema) bool {
	if rt == nil || attr == nil {
		return false
	}
	if attr.NS != 0 {
		nsBytes := rt.Namespaces.Bytes(attr.NS)
		return bytes.Equal(nsBytes, []byte(xmlnames.XMLNSNamespace))
	}
	return bytes.Equal(attr.NSBytes, []byte(xmlnames.XMLNSNamespace))
}

// AttrNamespaceMatches reports whether the attribute namespace matches the path op namespace.
func AttrNamespaceMatches(attr *Attr, ns runtime.NamespaceID, rt *runtime.Schema) bool {
	if attr == nil {
		return false
	}
	if attr.NS != 0 {
		return attr.NS == ns
	}
	if rt == nil {
		return false
	}
	return bytes.Equal(attr.NSBytes, rt.Namespaces.Bytes(ns))
}

// AttrNameMatches reports whether the attribute matches the path op QName.
func AttrNameMatches(attr *Attr, op runtime.PathOp, rt *runtime.Schema) bool {
	if attr == nil {
		return false
	}
	if attr.Sym != 0 {
		return attr.Sym == op.Sym
	}
	if rt == nil {
		return false
	}
	targetLocal := rt.Symbols.LocalBytes(op.Sym)
	if !bytes.Equal(attr.Local, targetLocal) {
		return false
	}
	return AttrNamespaceMatches(attr, op.NS, rt)
}
