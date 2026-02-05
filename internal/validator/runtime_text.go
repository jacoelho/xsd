package validator

import (
	"fmt"

	xsdErrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// TextState tracks character data for the current element.
type TextState struct {
	Off uint32
	Len uint32

	HasText  bool
	HasNonWS bool
}

func (s *Session) ResetText(state *TextState) {
	if s == nil || state == nil {
		return
	}
	state.Off = uint32(len(s.textBuf))
	state.Len = 0
	state.HasText = false
	state.HasNonWS = false
}

func (s *Session) ConsumeText(state *TextState, kind runtime.ContentKind, mixed, nilled bool, text []byte) error {
	if s == nil || state == nil {
		return fmt.Errorf("text state missing")
	}
	if len(text) == 0 {
		return nil
	}
	if nilled {
		return newValidationError(xsdErrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
	}
	kind = effectiveTextKind(kind, mixed)
	state.HasText = true
	if len(value.TrimXMLWhitespace(text)) > 0 {
		state.HasNonWS = true
	}

	switch kind {
	case runtime.ContentSimple:
		s.textBuf = append(s.textBuf, text...)
		state.Len += uint32(len(text))
		return nil
	case runtime.ContentMixed:
		return nil
	case runtime.ContentEmpty:
		return newValidationError(xsdErrors.ErrTextInElementOnly, "character data not allowed in empty content")
	case runtime.ContentElementOnly, runtime.ContentAll:
		if state.HasNonWS {
			return newValidationError(xsdErrors.ErrTextInElementOnly, "character data not allowed in element-only content")
		}
		return nil
	default:
		return fmt.Errorf("unknown content kind %d", kind)
	}
}

func (s *Session) TextSlice(state TextState) []byte {
	if s == nil {
		return nil
	}
	start := int(state.Off)
	end := start + int(state.Len)
	if start < 0 || end < start || end > len(s.textBuf) {
		return nil
	}
	return s.textBuf[start:end]
}

func (s *Session) releaseText(state TextState) {
	if s == nil {
		return
	}
	start := int(state.Off)
	end := start + int(state.Len)
	if start < 0 || end < start || end != len(s.textBuf) {
		return
	}
	s.textBuf = s.textBuf[:start]
}

func effectiveTextKind(kind runtime.ContentKind, mixed bool) runtime.ContentKind {
	if mixed {
		switch kind {
		case runtime.ContentElementOnly, runtime.ContentAll, runtime.ContentEmpty:
			return runtime.ContentMixed
		}
	}
	return kind
}
