package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) handleCharData(ev *xmlstream.ResolvedEvent) error {
	if ev == nil {
		return fmt.Errorf("character data event missing")
	}
	if len(s.elemStack) == 0 {
		return nil
	}
	frame := &s.elemStack[len(s.elemStack)-1]
	return s.ConsumeText(&frame.text, frame.content, frame.mixed, frame.nilled, ev.Text)
}
