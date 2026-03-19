package validator

import (
	"github.com/jacoelho/xsd/internal/validator/names"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) internName(id xmlstream.NameID, nsBytes, local []byte) names.Entry {
	if s == nil {
		return names.Entry{}
	}
	return s.Names.Intern(s.rt, id, nsBytes, local)
}
