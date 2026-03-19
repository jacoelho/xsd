package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/names"
)

func (s *Session) namespaceID(nsBytes []byte) runtime.NamespaceID {
	if s == nil {
		return 0
	}
	return names.NamespaceID(s.rt, nsBytes)
}
