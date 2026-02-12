package validator

import "github.com/jacoelho/xsd/internal/runtime"

func elementValueKey(frame *rtIdentityFrame, elem *runtime.Element, in identityEndInput) (runtime.ValueKind, []byte, bool) {
	if elem == nil {
		return runtime.VKInvalid, nil, false
	}
	if frame.nilled {
		return runtime.VKInvalid, nil, false
	}
	if in.KeyKind == runtime.VKInvalid {
		return runtime.VKInvalid, nil, true
	}
	return in.KeyKind, in.KeyBytes, true
}
