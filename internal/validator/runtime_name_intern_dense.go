package validator

import (
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) internName(id xmlstream.NameID, nsBytes, local []byte) NameEntry {
	if s == nil {
		return NameEntry{}
	}
	return s.Names.Intern(s.rt, id, nsBytes, local)
}
