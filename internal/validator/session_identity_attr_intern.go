package validator

import (
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/identity"
)

func (s *Session) internIdentityAttrName(ns, local []byte) identity.AttrNameID {
	if s == nil {
		return 0
	}
	return s.identityAttrs.Intern(attrs.NameHash(ns, local), ns, local)
}
