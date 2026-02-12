package validator

import "github.com/jacoelho/xsd/internal/runtime"

type defaultFixedPolicy struct {
	value   runtime.ValueRef
	key     runtime.ValueKeyRef
	member  runtime.ValidatorID
	fixed   bool
	present bool
}
