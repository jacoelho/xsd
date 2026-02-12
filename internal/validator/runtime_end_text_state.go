package validator

import "github.com/jacoelho/xsd/internal/runtime"

type endTextState struct {
	canonText     []byte
	textKeyBytes  []byte
	textValidator runtime.ValidatorID
	textMember    runtime.ValidatorID
	textKeyKind   runtime.ValueKind
}
