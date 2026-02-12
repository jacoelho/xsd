package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) ConsumeText(state *TextState, kind runtime.ContentKind, mixed, nilled bool, text []byte) error {
	if s == nil || state == nil {
		return fmt.Errorf("text state missing")
	}
	if len(text) == 0 {
		return nil
	}
	if nilled {
		return newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
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
		return newValidationError(xsderrors.ErrTextInElementOnly, "character data not allowed in empty content")
	case runtime.ContentElementOnly, runtime.ContentAll:
		if state.HasNonWS {
			return newValidationError(xsderrors.ErrTextInElementOnly, "character data not allowed in element-only content")
		}
		return nil
	default:
		return fmt.Errorf("unknown content kind %d", kind)
	}
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
