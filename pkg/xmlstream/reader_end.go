package xmlstream

import "github.com/jacoelho/xsd/pkg/xmltext"

type elementStackEntry struct {
	qname  QName
	nameID NameID
}

func (r *Reader) endEvent(_ *xmltext.RawTokenSpan, line, column int) (Event, error) {
	name, scopeDepth, err := r.beginEndElement()
	if err != nil {
		return Event{}, err
	}
	return Event{
		Kind:       EventEndElement,
		Name:       name.qname,
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) endRawEvent(tok *xmltext.RawTokenSpan, line, column int) (RawEvent, error) {
	_, scopeDepth, err := r.beginEndElement()
	if err != nil {
		return RawEvent{}, err
	}
	return RawEvent{
		Kind:       EventEndElement,
		Name:       rawNameFromBytes(tok.Name),
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) endResolvedEvent(tok *xmltext.RawTokenSpan, line, column int) (ResolvedEvent, error) {
	name, scopeDepth, err := r.beginEndElement()
	if err != nil {
		return ResolvedEvent{}, err
	}
	nameID := name.nameID
	if nameID == 0 {
		nameID = r.resolvedNameID(name.qname)
	}

	_, local, _ := splitQNameWithColon(tok.Name, tok.NameColon)
	namespace := name.qname.Namespace
	nsBytes := r.nsBytes.intern(namespace)
	return ResolvedEvent{
		Kind:       EventEndElement,
		NameID:     nameID,
		NS:         nsBytes,
		Local:      local,
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) beginEndElement() (elementStackEntry, int, error) {
	scopeDepth := r.ns.depth() - 1
	name, err := r.popElementName()
	if err != nil {
		return elementStackEntry{}, 0, err
	}
	r.pendingPop = true
	return name, scopeDepth, nil
}
