package validator

import "github.com/jacoelho/xsd/internal/validator/identity"

// SessionIdentity owns per-session identity-constraint state and caches.
type SessionIdentity struct {
	idTable       map[string]struct{}
	identityAttrs identity.AttrNames
	idRefs        []string
	icState       identityState
}
