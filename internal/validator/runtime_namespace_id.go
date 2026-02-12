package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) namespaceID(nsBytes []byte) runtime.NamespaceID {
	if len(nsBytes) == 0 {
		if s.rt != nil {
			return s.rt.PredefNS.Empty
		}
		return 0
	}
	if s.rt == nil {
		return 0
	}
	return s.rt.Namespaces.Lookup(nsBytes)
}
