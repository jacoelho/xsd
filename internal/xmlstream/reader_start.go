package xmlstream

import "github.com/jacoelho/xsd/internal/xmltext"

type startElementCore struct {
	namespace  string
	local      []byte
	scopeDepth int
}

func (r *Reader) startEvent(tok *xmltext.RawTokenSpan, line, column int) (Event, error) {
	core, err := r.beginStartElementCore(tok, line, column)
	if err != nil {
		return Event{}, err
	}
	name := r.names.internBytes(core.namespace, core.local)

	attrCount := tok.AttrCount()
	if cap(r.attrBuf) < attrCount {
		r.attrBuf = make([]Attr, 0, attrCount)
	} else {
		r.attrBuf = r.attrBuf[:0]
	}
	r.beginAttrDedup()

	err = r.scanStartAttributes(tok, core.scopeDepth, line, column, func(_ []byte, attrNamespace string, attrLocal, value []byte) error {
		if markErr := r.markAttrSeen(attrNamespace, attrLocal, line, column); markErr != nil {
			return markErr
		}
		r.attrBuf = append(r.attrBuf, Attr{
			Name:  r.names.internBytes(attrNamespace, attrLocal),
			Value: value,
		})
		return nil
	})
	if err != nil {
		return Event{}, err
	}

	_ = r.commitStartEvent(name, 0, line, column, core.scopeDepth)
	r.lastStart.Attrs = r.attrBuf
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return r.lastStart, nil
}

func (r *Reader) startRawEvent(tok *xmltext.RawTokenSpan, line, column int) (RawEvent, error) {
	core, err := r.beginStartElementCore(tok, line, column)
	if err != nil {
		return RawEvent{}, err
	}
	name := r.names.internBytes(core.namespace, core.local)

	attrCount := tok.AttrCount()
	r.attrBuf = r.attrBuf[:0]
	if cap(r.rawAttrBuf) < attrCount {
		r.rawAttrBuf = make([]RawAttr, 0, attrCount)
	} else {
		r.rawAttrBuf = r.rawAttrBuf[:0]
	}
	if cap(r.rawAttrInfo) < attrCount {
		r.rawAttrInfo = make([]rawAttrInfo, 0, attrCount)
	} else {
		r.rawAttrInfo = r.rawAttrInfo[:0]
	}
	r.beginAttrDedup()

	err = r.scanStartAttributes(tok, core.scopeDepth, line, column, func(attrName []byte, attrNamespace string, attrLocal, value []byte) error {
		if markErr := r.markAttrSeen(attrNamespace, attrLocal, line, column); markErr != nil {
			return markErr
		}
		r.rawAttrBuf = append(r.rawAttrBuf, RawAttr{
			Name:  rawNameFromBytes(attrName),
			Value: value,
		})
		r.rawAttrInfo = append(r.rawAttrInfo, rawAttrInfo{
			namespace: attrNamespace,
			local:     attrLocal,
		})
		return nil
	})
	if err != nil {
		return RawEvent{}, err
	}

	id := r.commitStartEvent(name, 0, line, column, core.scopeDepth)
	r.lastStart.Attrs = r.attrBuf
	r.lastRawAttrs = r.rawAttrBuf
	r.lastRawInfo = r.rawAttrInfo
	return RawEvent{
		Kind:       EventStartElement,
		Name:       rawNameFromBytes(tok.Name),
		Attrs:      r.rawAttrBuf,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: core.scopeDepth,
	}, nil
}

func (r *Reader) startResolvedEvent(tok *xmltext.RawTokenSpan, line, column int) (ResolvedEvent, error) {
	core, err := r.beginStartElementCore(tok, line, column)
	if err != nil {
		return ResolvedEvent{}, err
	}

	name := r.resolvedNames.internBytes(core.namespace, core.local)
	nsBytes := r.nsBytes.intern(core.namespace)

	attrCount := tok.AttrCount()
	if cap(r.resolvedAttr) < attrCount {
		r.resolvedAttr = make([]ResolvedAttr, 0, attrCount)
	} else {
		r.resolvedAttr = r.resolvedAttr[:0]
	}
	r.beginAttrDedup()

	err = r.scanStartAttributes(tok, core.scopeDepth, line, column, func(_ []byte, attrNamespace string, attrLocal, value []byte) error {
		attrID, attrErr := r.attrID(attrNamespace, attrLocal)
		if attrErr != nil {
			return wrapSyntaxError(r.dec, line, column, attrErr)
		}
		attrNSBytes := r.nsBytes.intern(attrNamespace)
		r.resolvedAttr = append(r.resolvedAttr, ResolvedAttr{
			NameID: attrID,
			NS:     attrNSBytes,
			Local:  attrLocal,
			Value:  value,
		})
		return nil
	})
	if err != nil {
		return ResolvedEvent{}, err
	}

	id := r.commitStartEvent(name.qname, name.id, line, column, core.scopeDepth)
	r.lastRawAttrs = nil
	r.lastRawInfo = nil

	return ResolvedEvent{
		Kind:       EventStartElement,
		NameID:     name.id,
		NS:         nsBytes,
		Local:      core.local,
		Attrs:      r.resolvedAttr,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: core.scopeDepth,
	}, nil
}

func (r *Reader) beginAttrDedup() {
	r.attrEpoch++
	if r.attrEpoch == 0 {
		clear(r.attrSeen)
		r.attrEpoch = 1
	}
}

func (r *Reader) attrID(namespace string, local []byte) (NameID, error) {
	entry := r.resolvedNames.internBytes(namespace, local)
	if entry.id == 0 {
		return 0, nil
	}
	idx := int(entry.id)
	if idx >= len(r.attrSeen) {
		r.attrSeen = append(r.attrSeen, make([]uint32, idx-len(r.attrSeen)+1)...)
	}
	if r.attrSeen[idx] == r.attrEpoch {
		return 0, errDuplicateAttribute
	}
	r.attrSeen[idx] = r.attrEpoch
	return entry.id, nil
}

func (r *Reader) resolvedNameID(name QName) NameID {
	if r.resolvedNames == nil {
		r.resolvedNames = newResolvedNameCache()
	}
	return r.resolvedNames.intern(name.Namespace, name.Local).id
}

func (r *Reader) markAttrSeen(namespace string, local []byte, line, column int) error {
	if r.resolvedNames == nil {
		r.resolvedNames = newResolvedNameCache()
	}
	if _, err := r.attrID(namespace, local); err != nil {
		return wrapSyntaxError(r.dec, line, column, err)
	}
	return nil
}

func (r *Reader) commitStartEvent(name QName, nameID NameID, line, column, scopeDepth int) ElementID {
	id := r.nextID
	r.nextID++
	r.elemStack = append(r.elemStack, elementStackEntry{
		qname:  name,
		nameID: nameID,
	})
	r.lastWasStart = true
	r.lastStart = Event{
		Kind:       EventStartElement,
		Name:       name,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: scopeDepth,
	}
	return id
}

func (r *Reader) beginStartElement(tok *xmltext.RawTokenSpan, line, column int) (int, error) {
	declStart := len(r.ns.decls)
	scope, nsBuf, decls, err := collectNamespaceScope(r.dec, r.nsBuf, r.ns.decls, tok)
	if err != nil {
		r.nsBuf = nsBuf
		return 0, wrapSyntaxError(r.dec, line, column, err)
	}
	r.nsBuf = nsBuf
	r.ns.decls = decls
	scope.declStart = declStart
	scope.declLen = len(r.ns.decls) - declStart
	return r.ns.push(scope), nil
}

func (r *Reader) beginStartElementCore(tok *xmltext.RawTokenSpan, line, column int) (startElementCore, error) {
	scopeDepth, err := r.beginStartElement(tok, line, column)
	if err != nil {
		return startElementCore{}, err
	}
	namespace, local, err := resolveElementParts(&r.ns, r.dec, tok.Name, tok.NameColon, scopeDepth, line, column)
	if err != nil {
		return startElementCore{}, err
	}
	return startElementCore{
		namespace:  namespace,
		local:      local,
		scopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) scanStartAttributes(
	tok *xmltext.RawTokenSpan,
	scopeDepth, line, column int,
	emit func(attrName []byte, namespace string, local, value []byte) error,
) error {
	for i := range tok.AttrCount() {
		attrName := tok.AttrName(i)
		if isDefaultNamespaceDecl(attrName) {
			continue
		}
		if _, ok := prefixedNamespaceDecl(attrName); ok {
			continue
		}
		attrNamespace, attrLocal, err := resolveAttrName(r.dec, &r.ns, attrName, tok.AttrNameColon(i), scopeDepth, line, column)
		if err != nil {
			return err
		}
		value, err := r.attrValueBytes(tok.AttrValue(i), tok.AttrValueNeeds(i))
		if err != nil {
			return wrapSyntaxError(r.dec, line, column, err)
		}
		if err := emit(attrName, attrNamespace, attrLocal, value); err != nil {
			return err
		}
	}
	return nil
}
