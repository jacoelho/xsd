package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/xmllex"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (e *validationExecutor) processCharData(ev *xmlstream.ResolvedEvent) error {
	if len(e.s.elemStack) == 0 {
		if !xmllex.IsIgnorableOutsideRoot(ev.Text, e.allowBOM) {
			if fatal := e.s.recordValidationError(fmt.Errorf("unexpected character data outside root element"), ev.Line, ev.Column); fatal != nil {
				return fatal
			}
		}
		e.allowBOM = false
		return nil
	}
	if err := e.s.handleCharData(ev); err != nil {
		if fatal := e.s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
			return fatal
		}
	}
	e.allowBOM = false
	return nil
}
