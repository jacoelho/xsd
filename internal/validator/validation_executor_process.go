package validator

import "github.com/jacoelho/xsd/pkg/xmlstream"

func (e *validationExecutor) process(ev *xmlstream.ResolvedEvent) error {
	if e == nil || e.s == nil || ev == nil {
		return nil
	}
	switch ev.Kind {
	case xmlstream.EventStartElement:
		return e.processStartElement(ev)
	case xmlstream.EventEndElement:
		return e.processEndElement(ev)
	case xmlstream.EventCharData:
		return e.processCharData(ev)
	default:
		return nil
	}
}
