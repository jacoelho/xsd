package identity

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// ApplyElementCaptures records one element value across all deferred field captures.
func ApplyElementCaptures(nilled bool, captures []FieldCapture, keyKind runtime.ValueKind, keyBytes []byte) {
	if len(captures) == 0 {
		return
	}
	kind, key, ok := ElementValueKey(nilled, keyKind, keyBytes)
	for _, capture := range captures {
		match := capture.Match
		if match == nil || match.Invalid {
			continue
		}
		fieldState := &match.Fields[capture.FieldIndex]
		if fieldState.Multiple || fieldState.Invalid {
			continue
		}
		if !ok {
			fieldState.Missing = true
			continue
		}
		if kind == runtime.VKInvalid {
			fieldState.Invalid = true
			continue
		}
		fieldState.KeyKind = kind
		fieldState.KeyBytes = append(fieldState.KeyBytes[:0], key...)
		fieldState.HasValue = true
	}
}

// CloseFrame applies deferred element captures, finalizes selector matches, and
// closes any scopes rooted at the finished frame.
func CloseFrame[F any](rt *runtime.Schema, arena ByteArena, state *State[F], frameID uint64, elemID runtime.ElemID, nilled bool, captures []FieldCapture, matches []*Match, keyKind runtime.ValueKind, keyBytes []byte) error {
	if _, ok := elementByID(rt, elemID); !ok {
		return fmt.Errorf("identity: element %d not found", elemID)
	}
	ApplyElementCaptures(nilled, captures, keyKind, keyBytes)
	FinalizeMatches(arena, matches)
	if state != nil {
		state.CloseScopes(frameID)
	}
	return nil
}
