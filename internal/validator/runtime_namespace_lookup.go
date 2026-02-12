package validator

import (
	"github.com/jacoelho/xsd/internal/xmlnames"
)

var xmlNamespaceBytes = xmlnames.XMLNamespaceBytes()

func (s *Session) lookupNamespace(prefix []byte) ([]byte, bool) {
	if isXMLPrefix(prefix) {
		return xmlNamespaceBytes, true
	}
	const smallNSDeclThreshold = 32
	frames := s.nsStack.Items()
	if len(s.nsDecls) <= smallNSDeclThreshold {
		return s.lookupNamespaceSmall(prefix, frames)
	}
	return s.lookupNamespaceHashed(prefix, frames)
}
