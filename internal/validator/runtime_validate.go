package validator

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	xsdxml "github.com/jacoelho/xsd/internal/xml"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

const maxNameMapSize = 1 << 20

var newXMLReader = xmlstream.NewReader

type sessionResolver struct {
	s *Session
}

func (r sessionResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	if r.s == nil {
		return nil, false
	}
	return r.s.lookupNamespace(prefix)
}

// Validate validates an XML document using the runtime schema.
func (s *Session) Validate(r io.Reader) error {
	if s == nil || s.rt == nil {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrSchemaNotLoaded, "schema not loaded", "")}
	}
	if r == nil {
		return readerSetupError(errors.New("nil reader"))
	}
	s.Reset()

	if s.reader == nil {
		reader, err := newXMLReader(r)
		if err != nil {
			return readerSetupError(err)
		}
		s.reader = reader
	} else if err := s.reader.Reset(r); err != nil {
		return readerSetupError(err)
	}

	rootSeen := false
	allowBOM := true
	resolver := sessionResolver{s: s}
	for {
		ev, err := s.reader.NextResolved()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, err.Error(), s.pathString())}
		}
		switch ev.Kind {
		case xmlstream.EventStartElement:
			if err := s.handleStartElement(&ev, resolver); err != nil {
				if fatal := s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
					return fatal
				}
				if skipErr := s.reader.SkipSubtree(); skipErr != nil {
					return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, skipErr.Error(), s.pathString())}
				}
			}
			if !rootSeen {
				rootSeen = true
			}
			allowBOM = false
		case xmlstream.EventEndElement:
			errs, path := s.handleEndElement(&ev, resolver)
			if len(errs) > 0 {
				if fatal := s.recordValidationErrorsAtPath(errs, path, ev.Line, ev.Column); fatal != nil {
					return fatal
				}
			}
			if len(s.icState.pending) > 0 {
				pending := s.icState.drainPending()
				if len(pending) > 0 {
					if fatal := s.recordValidationErrorsAtPath(pending, path, ev.Line, ev.Column); fatal != nil {
						return fatal
					}
				}
			}
			allowBOM = false
		case xmlstream.EventCharData:
			if len(s.elemStack) == 0 {
				if !xsdxml.IsIgnorableOutsideRoot(ev.Text, allowBOM) {
					if fatal := s.recordValidationError(fmt.Errorf("unexpected character data outside root element"), ev.Line, ev.Column); fatal != nil {
						return fatal
					}
				}
				allowBOM = false
				continue
			}
			if err := s.handleCharData(&ev); err != nil {
				if fatal := s.recordValidationError(err, ev.Line, ev.Column); fatal != nil {
					return fatal
				}
			}
			allowBOM = false
		}
	}

	if !rootSeen {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrNoRoot, "document has no root element", "")}
	}
	if len(s.elemStack) != 0 {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, "document ended with unclosed elements", s.pathString())}
	}
	if errs := s.validateIDRefs(); len(errs) > 0 {
		if fatal := s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	if errs := s.finalizeIdentity(); len(errs) > 0 {
		if fatal := s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	return s.validationList()
}

func readerSetupError(err error) error {
	if err == nil {
		return nil
	}
	return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, err.Error(), "")}
}

func (s *Session) handleStartElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) error {
	if ev == nil {
		return fmt.Errorf("start element event missing")
	}
	entry := s.internName(ev.NameID, ev.NS, ev.Local)
	sym := entry.Sym
	nsID := entry.NS

	decls := s.reader.NamespaceDeclsAt(ev.ScopeDepth)
	s.pushNamespaceScope(decls)

	var match StartMatch
	parentIndex := -1
	if len(s.elemStack) == 0 {
		switch s.rt.RootPolicy {
		case runtime.RootAny:
			if sym == 0 {
				if err := s.reader.SkipSubtree(); err != nil {
					s.popNamespaceScope()
					return err
				}
				s.popNamespaceScope()
				return nil
			}
			elemID, ok := s.globalElementBySymbol(sym)
			if !ok {
				if err := s.reader.SkipSubtree(); err != nil {
					s.popNamespaceScope()
					return err
				}
				s.popNamespaceScope()
				return nil
			}
			match = StartMatch{Kind: MatchElem, Elem: elemID}
		case runtime.RootStrict:
			if sym == 0 {
				s.popNamespaceScope()
				return newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
			}
			elemID, ok := s.globalElementBySymbol(sym)
			if !ok {
				s.popNamespaceScope()
				return newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
			}
			match = StartMatch{Kind: MatchElem, Elem: elemID}
		}
	} else {
		parentIndex = len(s.elemStack) - 1
		parent := &s.elemStack[parentIndex]
		if parent.nilled {
			s.popNamespaceScope()
			return newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
		}
		if parent.content == runtime.ContentSimple || parent.content == runtime.ContentEmpty {
			s.popNamespaceScope()
			if parent.content == runtime.ContentSimple {
				return newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
			}
			return newValidationError(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
		}
		if parent.model.Kind == runtime.ModelNone {
			s.popNamespaceScope()
			return newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
		}
		var err error
		match, err = s.StepModel(parent.model, &parent.modelState, sym, nsID, ev.NS)
		if err != nil {
			s.popNamespaceScope()
			return err
		}
	}

	attrs := s.makeStartAttrs(ev.Attrs)
	result, err := s.StartElement(match, sym, nsID, ev.NS, attrs, resolver)
	if err != nil {
		s.popNamespaceScope()
		return err
	}
	if result.Skip {
		if parentIndex >= 0 {
			s.elemStack[parentIndex].hasChildElements = true
		}
		err = s.reader.SkipSubtree()
		if err != nil {
			s.popNamespaceScope()
			return err
		}
		s.popNamespaceScope()
		return nil
	}

	attrResult, err := s.ValidateAttributes(result.Type, attrs, resolver)
	if err != nil {
		s.popNamespaceScope()
		return err
	}

	typ, ok := s.typeByID(result.Type)
	if !ok {
		s.popNamespaceScope()
		return fmt.Errorf("type %d not found", result.Type)
	}
	if parentIndex >= 0 {
		s.elemStack[parentIndex].hasChildElements = true
	}

	frame := elemFrame{
		name:   NameID(ev.NameID),
		elem:   result.Elem,
		typ:    result.Type,
		nilled: result.Nilled,
	}
	if entry.LocalLen == 0 && entry.NSLen == 0 {
		if len(ev.Local) > 0 {
			frame.local = append([]byte(nil), ev.Local...)
		}
		if len(ev.NS) > 0 {
			frame.ns = append([]byte(nil), ev.NS...)
		}
	}

	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		frame.content = runtime.ContentSimple
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			s.popNamespaceScope()
			return fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		frame.content = ct.Content
		frame.mixed = ct.Mixed
		frame.model = ct.Model
		if frame.model.Kind != runtime.ModelNone {
			state, err := s.InitModelState(frame.model)
			if err != nil {
				s.popNamespaceScope()
				return err
			}
			frame.modelState = state
		}
	default:
		s.popNamespaceScope()
		return fmt.Errorf("unknown type kind %d", typ.Kind)
	}

	s.ResetText(&frame.text)
	s.elemStack = append(s.elemStack, frame)

	if err := s.identityStart(identityStartInput{
		Elem:    result.Elem,
		Type:    result.Type,
		Sym:     sym,
		NS:      nsID,
		Attrs:   attrResult.Attrs,
		Applied: attrResult.Applied,
		Nilled:  result.Nilled,
	}); err != nil {
		s.releaseText(frame.text)
		s.elemStack = s.elemStack[:len(s.elemStack)-1]
		s.popNamespaceScope()
		return err
	}

	return nil
}

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

func (s *Session) handleEndElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) ([]error, string) {
	if ev == nil {
		return []error{fmt.Errorf("end element event missing")}, s.pathString()
	}
	if len(s.elemStack) == 0 {
		return []error{fmt.Errorf("unexpected end element")}, s.pathString()
	}
	index := len(s.elemStack) - 1
	frame := s.elemStack[index]

	var errs []error
	path := ""

	typ, ok := s.typeByID(frame.typ)
	if !ok {
		if path == "" {
			path = s.pathString()
		}
		errs = append(errs, fmt.Errorf("type %d not found", frame.typ))
	}
	elem, elemOK := s.element(frame.elem)
	if !elemOK {
		if path == "" {
			path = s.pathString()
		}
		errs = append(errs, fmt.Errorf("element %d out of range", frame.elem))
	}

	if frame.nilled {
		if frame.text.HasText || frame.hasChildElements {
			if path == "" {
				path = s.pathString()
			}
			errs = append(errs, newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty"))
		}
	} else {
		if frame.model.Kind != runtime.ModelNone {
			if err := s.AcceptModel(frame.model, &frame.modelState); err != nil {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, err)
			}
		}
	}

	var canonText []byte
	var textKeyKind runtime.ValueKind
	var textKeyBytes []byte
	textValidator := runtime.ValidatorID(0)
	if !frame.nilled && ok && (typ.Kind == runtime.TypeSimple || typ.Kind == runtime.TypeBuiltin || frame.content == runtime.ContentSimple) {
		if frame.hasChildElements {
			if path == "" {
				path = s.pathString()
			}
			errs = append(errs, newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content"))
		}
		rawText := s.TextSlice(frame.text)
		hasContent := frame.text.HasText || frame.hasChildElements
		var ct runtime.ComplexType
		hasComplexText := false
		if typ.Kind == runtime.TypeComplex {
			if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, fmt.Errorf("complex type %d missing", frame.typ))
			} else {
				ct = s.rt.ComplexTypes[typ.Complex.ID]
				hasComplexText = true
			}
		}
		switch typ.Kind {
		case runtime.TypeSimple, runtime.TypeBuiltin:
			textValidator = typ.Validator
		case runtime.TypeComplex:
			if hasComplexText {
				textValidator = ct.TextValidator
			}
		}
		trackDefault := func(value []byte) {
			if textValidator == 0 {
				return
			}
			if err := s.trackDefaultValue(textValidator, value); err != nil {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, err)
			}
		}
		switch {
		case !hasContent && elemOK && elem.Fixed.Present:
			canonText = valueBytes(s.rt.Values, elem.Fixed)
			trackDefault(canonText)
		case !hasContent && elemOK && elem.Default.Present:
			canonText = valueBytes(s.rt.Values, elem.Default)
			trackDefault(canonText)
		case !hasContent && hasComplexText && ct.TextFixed.Present:
			canonText = valueBytes(s.rt.Values, ct.TextFixed)
			trackDefault(canonText)
		case !hasContent && hasComplexText && ct.TextDefault.Present:
			canonText = valueBytes(s.rt.Values, ct.TextDefault)
			trackDefault(canonText)
		default:
			requireCanonical := (elemOK && elem.Fixed.Present) || (hasComplexText && ct.TextFixed.Present)
			canon, metrics, err := s.ValidateTextValue(frame.typ, rawText, resolver, requireCanonical)
			if err != nil {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, err)
			} else {
				canonText = canon
				textKeyKind = metrics.keyKind
				textKeyBytes = metrics.keyBytes
			}
		}
		if canonText != nil && elemOK && (frame.text.HasText || hasContent) && elem.Fixed.Present {
			fixed := valueBytes(s.rt.Values, elem.Fixed)
			if !bytes.Equal(canonText, fixed) {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch"))
			}
		} else if canonText != nil && (frame.text.HasText || hasContent) && hasComplexText && ct.TextFixed.Present && (!elemOK || !elem.Fixed.Present) {
			fixed := valueBytes(s.rt.Values, ct.TextFixed)
			if !bytes.Equal(canonText, fixed) {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch"))
			}
		}
	}

	if s.hasIdentityConstraints() && textKeyKind == runtime.VKInvalid && canonText != nil && textValidator != 0 {
		kind, key, err := s.keyForCanonicalValue(textValidator, canonText)
		if err != nil {
			if path == "" {
				path = s.pathString()
			}
			errs = append(errs, err)
		} else {
			textKeyKind = kind
			textKeyBytes = s.storeKey(key)
		}
	}

	if err := s.identityEnd(identityEndInput{
		Text:      canonText,
		TextState: frame.text,
		KeyKind:   textKeyKind,
		KeyBytes:  textKeyBytes,
	}); err != nil {
		if path == "" {
			path = s.pathString()
		}
		errs = append(errs, err)
	}

	if path == "" && len(s.icState.pending) > 0 {
		path = s.pathString()
	}

	s.releaseText(frame.text)
	s.elemStack = s.elemStack[:index]
	s.popNamespaceScope()
	return errs, path
}

func (s *Session) pushNamespaceScope(decls []xmlstream.NamespaceDecl) {
	off := len(s.nsDecls)
	cacheOff := len(s.prefixCache)
	for _, decl := range decls {
		prefixBytes := []byte(decl.Prefix)
		nsBytes := []byte(decl.URI)
		prefixOff := len(s.nameLocal)
		s.nameLocal = append(s.nameLocal, prefixBytes...)
		nsOff := len(s.nameNS)
		s.nameNS = append(s.nameNS, nsBytes...)
		s.nsDecls = append(s.nsDecls, nsDecl{
			prefixOff:  uint32(prefixOff),
			prefixLen:  uint32(len(prefixBytes)),
			nsOff:      uint32(nsOff),
			nsLen:      uint32(len(nsBytes)),
			prefixHash: runtime.HashBytes(prefixBytes),
		})
	}
	s.nsStack = append(s.nsStack, nsFrame{off: uint32(off), len: uint32(len(decls)), cacheOff: uint32(cacheOff)})
}

func (s *Session) popNamespaceScope() {
	if len(s.nsStack) == 0 {
		return
	}
	frame := s.nsStack[len(s.nsStack)-1]
	s.nsStack = s.nsStack[:len(s.nsStack)-1]
	if int(frame.off) <= len(s.nsDecls) {
		s.nsDecls = s.nsDecls[:frame.off]
	}
	if int(frame.cacheOff) <= len(s.prefixCache) {
		s.prefixCache = s.prefixCache[:frame.cacheOff]
	}
}

func (s *Session) lookupNamespace(prefix []byte) ([]byte, bool) {
	if bytes.Equal(prefix, []byte("xml")) {
		return []byte(xmlstream.XMLNamespace), true
	}
	const smallNSDeclThreshold = 32
	if len(s.nsDecls) <= smallNSDeclThreshold {
		if len(prefix) == 0 {
			for i := len(s.nsStack) - 1; i >= 0; i-- {
				frame := s.nsStack[i]
				for j := int(frame.off + frame.len); j > int(frame.off); j-- {
					decl := s.nsDecls[j-1]
					if decl.prefixLen != 0 {
						continue
					}
					return s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen], true
				}
			}
			return nil, true
		}
		for i := len(s.nsStack) - 1; i >= 0; i-- {
			frame := s.nsStack[i]
			for j := int(frame.off + frame.len); j > int(frame.off); j-- {
				decl := s.nsDecls[j-1]
				if decl.prefixLen == 0 {
					continue
				}
				prefixBytes := s.nameLocal[decl.prefixOff : decl.prefixOff+decl.prefixLen]
				if bytes.Equal(prefixBytes, prefix) {
					return s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen], true
				}
			}
		}
		return nil, false
	}

	hash := runtime.HashBytes(prefix)
	if cache := s.prefixCacheForCurrent(); len(cache) > 0 {
		for i := range cache {
			entry := &cache[i]
			if entry.hash != hash {
				continue
			}
			if entry.prefixLen == 0 {
				if len(prefix) != 0 {
					continue
				}
				if entry.ok {
					if entry.nsLen == 0 {
						return nil, true
					}
					return s.nameNS[entry.nsOff : entry.nsOff+entry.nsLen], true
				}
				return nil, false
			}
			if len(prefix) != int(entry.prefixLen) {
				continue
			}
			prefixBytes := s.nameLocal[entry.prefixOff : entry.prefixOff+entry.prefixLen]
			if !bytes.Equal(prefixBytes, prefix) {
				continue
			}
			if entry.ok {
				if entry.nsLen == 0 {
					return nil, true
				}
				return s.nameNS[entry.nsOff : entry.nsOff+entry.nsLen], true
			}
			return nil, false
		}
	}
	for i := len(s.nsStack) - 1; i >= 0; i-- {
		frame := s.nsStack[i]
		for j := int(frame.off + frame.len); j > int(frame.off); j-- {
			decl := s.nsDecls[j-1]
			if decl.prefixHash != hash {
				continue
			}
			if decl.prefixLen == 0 {
				if len(prefix) != 0 {
					continue
				}
				ns := s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen]
				s.cachePrefixDecl(decl, true, hash)
				return ns, true
			}
			if len(prefix) != int(decl.prefixLen) {
				continue
			}
			prefixBytes := s.nameLocal[decl.prefixOff : decl.prefixOff+decl.prefixLen]
			if !bytes.Equal(prefixBytes, prefix) {
				continue
			}
			ns := s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen]
			s.cachePrefixDecl(decl, true, hash)
			return ns, true
		}
	}
	if len(prefix) == 0 {
		s.cachePrefix(prefix, nil, true, hash)
		return nil, true
	}
	s.cachePrefix(prefix, nil, false, hash)
	return nil, false
}

func (s *Session) prefixCacheForCurrent() []prefixEntry {
	if len(s.nsStack) == 0 {
		return nil
	}
	frame := s.nsStack[len(s.nsStack)-1]
	if int(frame.cacheOff) >= len(s.prefixCache) {
		return nil
	}
	return s.prefixCache[frame.cacheOff:]
}

func (s *Session) cachePrefix(prefix, ns []byte, ok bool, hash uint64) {
	if s == nil {
		return
	}
	prefixLen := len(prefix)
	prefixOff := 0
	if prefixLen > 0 {
		prefixOff = len(s.nameLocal)
		s.nameLocal = append(s.nameLocal, prefix...)
	}
	nsLen := len(ns)
	nsOff := 0
	if ok && nsLen > 0 {
		nsOff = len(s.nameNS)
		s.nameNS = append(s.nameNS, ns...)
	}
	s.prefixCache = append(s.prefixCache, prefixEntry{
		hash:      hash,
		prefixOff: uint32(prefixOff),
		prefixLen: uint32(prefixLen),
		nsOff:     uint32(nsOff),
		nsLen:     uint32(nsLen),
		ok:        ok,
	})
}

func (s *Session) cachePrefixDecl(decl nsDecl, ok bool, hash uint64) {
	if s == nil {
		return
	}
	s.prefixCache = append(s.prefixCache, prefixEntry{
		hash:      hash,
		prefixOff: decl.prefixOff,
		prefixLen: decl.prefixLen,
		nsOff:     decl.nsOff,
		nsLen:     decl.nsLen,
		ok:        ok,
	})
}

func (s *Session) internName(id xmlstream.NameID, nsBytes, local []byte) nameEntry {
	if id == 0 {
		return nameEntry{Sym: 0, NS: s.namespaceID(nsBytes)}
	}
	idx := int(id)
	if idx >= maxNameMapSize {
		return s.internSparseName(NameID(id), nsBytes, local)
	}
	if idx >= len(s.nameMap) {
		s.nameMap = append(s.nameMap, make([]nameEntry, idx-len(s.nameMap)+1)...)
	}
	entry := s.nameMap[idx]
	if entry.LocalLen != 0 || entry.NSLen != 0 || entry.Sym != 0 || entry.NS != 0 {
		return entry
	}
	localOff := len(s.nameLocal)
	s.nameLocal = append(s.nameLocal, local...)
	nsOff := len(s.nameNS)
	s.nameNS = append(s.nameNS, nsBytes...)
	nsID := s.namespaceID(nsBytes)
	sym := runtime.SymbolID(0)
	if nsID != 0 {
		sym = s.rt.Symbols.Lookup(nsID, local)
	}
	entry = nameEntry{
		Sym:      sym,
		NS:       nsID,
		LocalOff: uint32(localOff),
		LocalLen: uint32(len(local)),
		NSOff:    uint32(nsOff),
		NSLen:    uint32(len(nsBytes)),
	}
	s.nameMap[idx] = entry
	return entry
}

func (s *Session) internSparseName(id NameID, nsBytes, local []byte) nameEntry {
	if s == nil {
		return nameEntry{}
	}
	if s.nameMapSparse == nil {
		s.nameMapSparse = make(map[NameID]nameEntry)
	}
	if entry, ok := s.nameMapSparse[id]; ok {
		return entry
	}
	if len(s.nameMapSparse) >= maxNameMapSize {
		nsID := s.namespaceID(nsBytes)
		sym := runtime.SymbolID(0)
		if nsID != 0 {
			sym = s.rt.Symbols.Lookup(nsID, local)
		}
		return nameEntry{Sym: sym, NS: nsID}
	}
	localOff := len(s.nameLocal)
	s.nameLocal = append(s.nameLocal, local...)
	nsOff := len(s.nameNS)
	s.nameNS = append(s.nameNS, nsBytes...)
	nsID := s.namespaceID(nsBytes)
	sym := runtime.SymbolID(0)
	if nsID != 0 {
		sym = s.rt.Symbols.Lookup(nsID, local)
	}
	entry := nameEntry{
		Sym:      sym,
		NS:       nsID,
		LocalOff: uint32(localOff),
		LocalLen: uint32(len(local)),
		NSOff:    uint32(nsOff),
		NSLen:    uint32(len(nsBytes)),
	}
	s.nameMapSparse[id] = entry
	return entry
}

func (s *Session) namespaceID(nsBytes []byte) runtime.NamespaceID {
	if len(nsBytes) == 0 {
		if s.rt != nil {
			return s.rt.PredefNS.Empty
		}
		return 0
	}
	if s.rt == nil {
		return 0
	}
	return s.rt.Namespaces.Lookup(nsBytes)
}

func (s *Session) makeStartAttrs(attrs []xmlstream.ResolvedAttr) []StartAttr {
	if len(attrs) == 0 {
		return nil
	}
	out := s.attrBuf[:0]
	if cap(out) < len(attrs) {
		out = make([]StartAttr, 0, len(attrs))
	}
	for _, attr := range attrs {
		entry := s.internName(attr.NameID, attr.NS, attr.Local)
		local := attr.Local
		nsBytes := attr.NS
		nameCached := false
		if entry.LocalLen != 0 {
			local = s.nameLocal[entry.LocalOff : entry.LocalOff+entry.LocalLen]
			nameCached = true
		}
		if entry.NSLen != 0 {
			nsBytes = s.nameNS[entry.NSOff : entry.NSOff+entry.NSLen]
			nameCached = true
		}
		out = append(out, StartAttr{
			Sym:        entry.Sym,
			NS:         entry.NS,
			NSBytes:    nsBytes,
			Local:      local,
			NameCached: nameCached,
			Value:      attr.Value,
		})
	}
	s.attrBuf = out[:0]
	return out
}

func (s *Session) finalizeIdentity() []error {
	if s == nil {
		return nil
	}
	if len(s.icState.violations) > 0 {
		errs := append([]error(nil), s.icState.violations...)
		s.icState.violations = s.icState.violations[:0]
		return errs
	}
	if pending := s.icState.drainPending(); len(pending) > 0 {
		return pending
	}
	return nil
}
