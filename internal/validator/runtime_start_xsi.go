package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

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
