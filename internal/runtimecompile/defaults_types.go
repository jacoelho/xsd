package runtimecompile

import "github.com/jacoelho/xsd/internal/runtime"

type compiledDefaultFixed struct {
	key    runtime.ValueKeyRef
	ref    runtime.ValueRef
	member runtime.ValidatorID
	ok     bool
}
