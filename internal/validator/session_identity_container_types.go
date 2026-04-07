package validator

// SessionIdentity owns per-session identity-constraint state and caches.
type SessionIdentity struct {
	idTable       map[string]struct{}
	identityAttrs AttrNames
	idRefs        []string
	icState       identityState
}
