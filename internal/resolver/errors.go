package resolver

import "errors"

// ErrTypeNotFound indicates a missing type reference during resolution.
var ErrTypeNotFound = errors.New("type not found")
