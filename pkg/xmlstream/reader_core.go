package xmlstream

import "github.com/jacoelho/xsd/pkg/xmltext"

type nextMode uint8

const (
	nextEvent nextMode = iota
	nextRaw
	nextResolved
)

func (r *Reader) next(mode nextMode, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	if r == nil || r.dec == nil {
		return errNilReader
	}
	if r.names == nil {
		r.names = newQNameCache()
	}
	if r.resolvedNames == nil {
		r.resolvedNames = newResolvedNameCache()
	}
	if r.pendingPop {
		r.ns.pop()
		r.pendingPop = false
	}
	r.lastWasStart = false

	for {
		if err := r.dec.ReadTokenRawSpansInto(&r.tok); err != nil {
			return err
		}
		tok := &r.tok
		line, column := tok.Line, tok.Column
		r.lastLine = line
		r.lastColumn = column
		r.valueBuf = r.valueBuf[:0]

		switch tok.Kind {
		case xmltext.KindStartElement:
			return r.emitStart(mode, tok, line, column, event, raw, resolved)
		case xmltext.KindEndElement:
			return r.emitEnd(mode, tok, line, column, event, raw, resolved)
		case xmltext.KindPI:
			if tok.IsXMLDecl {
				continue
			}
			fallthrough
		case xmltext.KindCharData, xmltext.KindCDATA, xmltext.KindComment, xmltext.KindDirective:
			return r.emitTokenText(mode, tok, line, column, event, raw, resolved)
		}
	}
}

func (r *Reader) emitStart(mode nextMode, tok *xmltext.RawTokenSpan, line, column int, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	return emitProjectedEvent(
		mode,
		event,
		raw,
		resolved,
		func() (Event, error) { return r.startEvent(tok, line, column) },
		func() (RawEvent, error) { return r.startRawEvent(tok, line, column) },
		func() (ResolvedEvent, error) { return r.startResolvedEvent(tok, line, column) },
	)
}

func (r *Reader) emitEnd(mode nextMode, tok *xmltext.RawTokenSpan, line, column int, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	return emitProjectedEvent(
		mode,
		event,
		raw,
		resolved,
		func() (Event, error) { return r.endEvent(tok, line, column) },
		func() (RawEvent, error) { return r.endRawEvent(tok, line, column) },
		func() (ResolvedEvent, error) { return r.endResolvedEvent(tok, line, column) },
	)
}

func emitProjectedEvent(
	mode nextMode,
	event *Event,
	raw *RawEvent,
	resolved *ResolvedEvent,
	eventFn func() (Event, error),
	rawFn func() (RawEvent, error),
	resolvedFn func() (ResolvedEvent, error),
) error {
	switch mode {
	case nextEvent:
		return assignProjectedEvent(event, eventFn)
	case nextResolved:
		return assignProjectedEvent(resolved, resolvedFn)
	default:
		return assignProjectedEvent(raw, rawFn)
	}
}

func assignProjectedEvent[T any](dst *T, build func() (T, error)) error {
	value, err := build()
	if err != nil {
		return err
	}
	if dst != nil {
		*dst = value
	}
	return nil
}
