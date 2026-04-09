package xmlstream

import "github.com/jacoelho/xsd/pkg/xmltext"

func (r *Reader) emitTokenText(mode nextMode, tok *xmltext.RawTokenSpan, line, column int, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	kind, ok := tokenEventKind(tok.Kind)
	if !ok {
		return nil
	}
	text, err := r.tokenText(tok)
	if err != nil {
		return wrapSyntaxError(r.dec, line, column, err)
	}
	scopeDepth := r.currentScopeDepth()
	r.projectTokenEvent(mode, kind, text, line, column, scopeDepth, event, raw, resolved)
	return nil
}

func tokenEventKind(kind xmltext.Kind) (EventKind, bool) {
	switch kind {
	case xmltext.KindCharData, xmltext.KindCDATA:
		return EventCharData, true
	case xmltext.KindComment:
		return EventComment, true
	case xmltext.KindPI:
		return EventPI, true
	case xmltext.KindDirective:
		return EventDirective, true
	default:
		return 0, false
	}
}

func (r *Reader) tokenText(tok *xmltext.RawTokenSpan) ([]byte, error) {
	switch tok.Kind {
	case xmltext.KindCharData, xmltext.KindCDATA:
		return r.textBytes(tok.Text, tok.TextNeeds)
	default:
		return tok.Text, nil
	}
}

func (r *Reader) projectTokenEvent(mode nextMode, kind EventKind, text []byte, line, column, scopeDepth int, event *Event, raw *RawEvent, resolved *ResolvedEvent) {
	switch mode {
	case nextEvent:
		if event != nil {
			*event = Event{
				Kind:       kind,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}
		}
	case nextResolved:
		if resolved != nil {
			*resolved = ResolvedEvent{
				Kind:       kind,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}
		}
	default:
		if raw != nil {
			*raw = RawEvent{
				Kind:       kind,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}
		}
	}
}
