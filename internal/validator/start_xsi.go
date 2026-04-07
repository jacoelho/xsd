package validator

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// ResolveStartXSIType resolves one xsi:type lexical QName to a runtime type ID.
func ResolveStartXSIType(rt *runtime.Schema, valueBytes []byte, resolver value.NSResolver) (runtime.TypeID, error) {
	if rt == nil {
		return 0, fmt.Errorf("runtime schema missing")
	}

	canonicalValue, err := value.CanonicalQName(valueBytes, resolver, nil)
	if err != nil {
		return 0, fmt.Errorf("resolve xsi:type: %w", err)
	}
	ns, local, err := splitCanonicalQName(canonicalValue)
	if err != nil {
		return 0, err
	}

	var nsID runtime.NamespaceID
	if len(ns) == 0 {
		nsID = rt.PredefNS.Empty
	} else {
		nsID = rt.Namespaces.Lookup(ns)
		if nsID == 0 {
			return 0, fmt.Errorf("xsi:type namespace not found")
		}
	}

	sym := rt.Symbols.Lookup(nsID, local)
	if sym == 0 {
		return 0, fmt.Errorf("xsi:type symbol not found")
	}

	id, ok := typeBySymbolID(rt, sym)
	if !ok {
		return 0, fmt.Errorf("xsi:type %d not found", sym)
	}
	return id, nil
}

func typeBySymbolID(rt *runtime.Schema, sym runtime.SymbolID) (runtime.TypeID, bool) {
	if sym == 0 {
		return 0, false
	}
	if rt == nil || int(sym) >= len(rt.GlobalTypes) {
		return 0, false
	}
	id := rt.GlobalTypes[sym]
	return id, id != 0
}

func splitCanonicalQName(canonical []byte) ([]byte, []byte, error) {
	for i, b := range canonical {
		if b == 0 {
			return canonical[:i], canonical[i+1:], nil
		}
	}
	return nil, nil, errors.New("invalid canonical QName")
}
