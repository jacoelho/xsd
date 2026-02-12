package validator

// SessionIdentity owns per-session identity-constraint state and caches.
type SessionIdentity struct {
	idTable             map[string]struct{}
	identityAttrBuckets map[uint64][]identityAttrNameID
	idRefs              []string
	identityAttrNames   []identityAttrName
	icState             identityState
}

type identityAttrName struct {
	ns    []byte
	local []byte
}
