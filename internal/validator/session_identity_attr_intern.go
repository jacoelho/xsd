package validator

func (s *Session) internIdentityAttrName(ns, local []byte) AttrNameID {
	if s == nil {
		return 0
	}
	return s.identityAttrs.Intern(NameHash(ns, local), ns, local)
}
