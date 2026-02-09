package semanticresolve

import "errors"

type idValuePolicy int

const (
	idValuesAllowed idValuePolicy = iota
	idValuesDisallowed
)

var errCircularReference = errors.New("circular type reference")
