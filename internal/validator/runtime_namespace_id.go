package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) namespaceID(nsBytes []byte) runtime.NamespaceID {
	if s == nil {
		return 0
	}
	return schemaNamespaceID(s.rt, nsBytes)
}
