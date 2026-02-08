package validator

import (
	"bytes"
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) scanXsiAttributes(attrs []StartAttr) ([]byte, []byte, error) {
	var xsiType []byte
	var xsiNil []byte
	predef := s.rt.Predef
	xsiNS := s.rt.PredefNS.Xsi
	xsiNSBytes := s.rt.Namespaces.Bytes(xsiNS)
	typeLocal := []byte("type")
	nilLocal := []byte("nil")
	for _, attr := range attrs {
		switch attr.Sym {
		case predef.XsiType:
			if len(xsiType) > 0 {
				return nil, nil, newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
			}
			xsiType = attr.Value
		case predef.XsiNil:
			if len(xsiNil) > 0 {
				return nil, nil, newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
			}
			xsiNil = attr.Value
			continue
		}
		if attr.Sym != 0 {
			continue
		}
		if !isXsiNamespace(attr, xsiNS, xsiNSBytes) {
			continue
		}
		if bytes.Equal(attr.Local, typeLocal) {
			if len(xsiType) > 0 {
				return nil, nil, newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
			}
			xsiType = attr.Value
			continue
		}
		if bytes.Equal(attr.Local, nilLocal) {
			if len(xsiNil) > 0 {
				return nil, nil, newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
			}
			xsiNil = attr.Value
		}
	}
	return xsiType, xsiNil, nil
}

func isXsiNamespace(attr StartAttr, xsiNS runtime.NamespaceID, xsiNSBytes []byte) bool {
	if attr.NS != 0 {
		return attr.NS == xsiNS
	}
	if len(attr.NSBytes) == 0 || len(xsiNSBytes) == 0 {
		return false
	}
	return bytes.Equal(attr.NSBytes, xsiNSBytes)
}

func (s *Session) resolveXsiType(valueBytes []byte, resolver value.NSResolver) (runtime.TypeID, error) {
	canonical, err := value.CanonicalQName(valueBytes, resolver, nil)
	if err != nil {
		return 0, fmt.Errorf("resolve xsi:type: %w", err)
	}
	ns, local, err := splitCanonicalQName(canonical)
	if err != nil {
		return 0, err
	}
	var nsID runtime.NamespaceID
	if len(ns) == 0 {
		nsID = s.rt.PredefNS.Empty
	} else {
		nsID = s.rt.Namespaces.Lookup(ns)
		if nsID == 0 {
			return 0, fmt.Errorf("xsi:type namespace not found")
		}
	}
	sym := s.rt.Symbols.Lookup(nsID, local)
	if sym == 0 {
		return 0, fmt.Errorf("xsi:type symbol not found")
	}
	id, ok := s.typeBySymbolID(sym)
	if !ok {
		return 0, fmt.Errorf("xsi:type %d not found", sym)
	}
	return id, nil
}

func splitCanonicalQName(valueBytes []byte) ([]byte, []byte, error) {
	for i, b := range valueBytes {
		if b == 0 {
			return valueBytes[:i], valueBytes[i+1:], nil
		}
	}
	return nil, nil, fmt.Errorf("invalid canonical QName")
}
