package validator

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/ic"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

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
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, "nil reader", "")}
	}
	s.Reset()

	if s.reader == nil {
		reader, err := xmlstream.NewReader(r)
		if err != nil {
			return err
		}
		s.reader = reader
	} else if err := s.reader.Reset(r); err != nil {
		return err
	}

	rootSeen := false
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
				return s.wrapValidationError(err, ev.Line, ev.Column)
			}
			if !rootSeen {
				rootSeen = true
			}
		case xmlstream.EventEndElement:
			if err := s.handleEndElement(&ev, resolver); err != nil {
				return s.wrapValidationError(err, ev.Line, ev.Column)
			}
		case xmlstream.EventCharData:
			if err := s.handleCharData(&ev); err != nil {
				return s.wrapValidationError(err, ev.Line, ev.Column)
			}
		}
	}

	if !rootSeen {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrNoRoot, "document has no root element", "")}
	}
	if len(s.elemStack) != 0 {
		return xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, "document ended with unclosed elements", s.pathString())}
	}
	if err := s.validateIDRefs(); err != nil {
		return s.wrapValidationError(err, 0, 0)
	}
	if err := s.finalizeIdentity(); err != nil {
		return s.wrapValidationError(err, 0, 0)
	}
	return nil
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
	if len(s.elemStack) == 0 {
		if s.rt.RootPolicy == runtime.RootStrict || s.rt.RootPolicy == runtime.RootAny {
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
		parent := &s.elemStack[len(s.elemStack)-1]
		parent.hasChildElements = true
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

	frame := elemFrame{
		name:   NameID(ev.NameID),
		elem:   result.Elem,
		typ:    result.Type,
		nilled: result.Nilled,
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

func (s *Session) handleEndElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) error {
	if ev == nil {
		return fmt.Errorf("end element event missing")
	}
	if len(s.elemStack) == 0 {
		return fmt.Errorf("unexpected end element")
	}
	index := len(s.elemStack) - 1
	frame := s.elemStack[index]

	typ, ok := s.typeByID(frame.typ)
	if !ok {
		s.popNamespaceScope()
		return fmt.Errorf("type %d not found", frame.typ)
	}

	if frame.nilled {
		if frame.text.HasText || frame.hasChildElements {
			s.popNamespaceScope()
			return newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
		}
	} else {
		if frame.model.Kind != runtime.ModelNone {
			if err := s.AcceptModel(frame.model, &frame.modelState); err != nil {
				s.popNamespaceScope()
				return err
			}
		}
	}

	var canonText []byte
	if !frame.nilled && (typ.Kind == runtime.TypeSimple || typ.Kind == runtime.TypeBuiltin || frame.content == runtime.ContentSimple) {
		if frame.hasChildElements {
			s.popNamespaceScope()
			return newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
		}
		rawText := s.TextSlice(frame.text)
		hasContent := frame.text.HasText || frame.hasChildElements
		var ct runtime.ComplexType
		hasComplexText := false
		if typ.Kind == runtime.TypeComplex {
			if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
				s.popNamespaceScope()
				return fmt.Errorf("complex type %d missing", frame.typ)
			}
			ct = s.rt.ComplexTypes[typ.Complex.ID]
			hasComplexText = true
		}
		textValidator := runtime.ValidatorID(0)
		switch typ.Kind {
		case runtime.TypeSimple, runtime.TypeBuiltin:
			textValidator = typ.Validator
		case runtime.TypeComplex:
			if hasComplexText {
				textValidator = ct.TextValidator
			}
		}
		trackDefault := func(value []byte) error {
			if textValidator == 0 {
				return nil
			}
			return s.trackDefaultValue(textValidator, value)
		}
		elem, _ := s.element(frame.elem)
		switch {
		case !hasContent && elem.Fixed.Present:
			canonText = valueBytes(s.rt.Values, elem.Fixed)
			if err := trackDefault(canonText); err != nil {
				s.popNamespaceScope()
				return err
			}
		case !hasContent && elem.Default.Present:
			canonText = valueBytes(s.rt.Values, elem.Default)
			if err := trackDefault(canonText); err != nil {
				s.popNamespaceScope()
				return err
			}
		case !hasContent && hasComplexText && ct.TextFixed.Present:
			canonText = valueBytes(s.rt.Values, ct.TextFixed)
			if err := trackDefault(canonText); err != nil {
				s.popNamespaceScope()
				return err
			}
		case !hasContent && hasComplexText && ct.TextDefault.Present:
			canonText = valueBytes(s.rt.Values, ct.TextDefault)
			if err := trackDefault(canonText); err != nil {
				s.popNamespaceScope()
				return err
			}
		default:
			requireCanonical := elem.Fixed.Present || (hasComplexText && ct.TextFixed.Present)
			canon, err := s.ValidateTextValue(frame.typ, rawText, resolver, requireCanonical)
			if err != nil {
				s.popNamespaceScope()
				return err
			}
			canonText = canon
		}
		if (frame.text.HasText || hasContent) && elem.Fixed.Present {
			fixed := valueBytes(s.rt.Values, elem.Fixed)
			if !bytes.Equal(canonText, fixed) {
				s.popNamespaceScope()
				return newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch")
			}
		} else if (frame.text.HasText || hasContent) && hasComplexText && ct.TextFixed.Present && !elem.Fixed.Present {
			fixed := valueBytes(s.rt.Values, ct.TextFixed)
			if !bytes.Equal(canonText, fixed) {
				s.popNamespaceScope()
				return newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch")
			}
		}
	}

	if err := s.identityEnd(identityEndInput{
		Text:      canonText,
		TextState: frame.text,
	}); err != nil {
		s.popNamespaceScope()
		return err
	}

	s.releaseText(frame.text)
	s.elemStack = s.elemStack[:index]
	s.popNamespaceScope()
	return nil
}

func (s *Session) pushNamespaceScope(decls []xmlstream.NamespaceDecl) {
	off := len(s.nsDecls)
	for _, decl := range decls {
		prefixBytes := []byte(decl.Prefix)
		nsBytes := []byte(decl.URI)
		prefixOff := len(s.nameLocal)
		s.nameLocal = append(s.nameLocal, prefixBytes...)
		nsOff := len(s.nameNS)
		s.nameNS = append(s.nameNS, nsBytes...)
		s.nsDecls = append(s.nsDecls, nsDecl{
			prefixOff: uint32(prefixOff),
			prefixLen: uint32(len(prefixBytes)),
			nsOff:     uint32(nsOff),
			nsLen:     uint32(len(nsBytes)),
		})
	}
	s.nsStack = append(s.nsStack, nsFrame{off: uint32(off), len: uint32(len(decls))})
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
}

func (s *Session) lookupNamespace(prefix []byte) ([]byte, bool) {
	if bytes.Equal(prefix, []byte("xml")) {
		return []byte(xmlstream.XMLNamespace), true
	}
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

func (s *Session) internName(id xmlstream.NameID, nsBytes, local []byte) nameEntry {
	if id == 0 {
		return nameEntry{Sym: 0, NS: s.namespaceID(nsBytes)}
	}
	idx := int(id)
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
		if entry.LocalLen != 0 {
			local = s.nameLocal[entry.LocalOff : entry.LocalOff+entry.LocalLen]
		}
		if entry.NSLen != 0 {
			nsBytes = s.nameNS[entry.NSOff : entry.NSOff+entry.NSLen]
		}
		out = append(out, StartAttr{
			Sym:     entry.Sym,
			NS:      entry.NS,
			NSBytes: nsBytes,
			Local:   local,
			Value:   attr.Value,
		})
	}
	s.attrBuf = out[:0]
	return out
}

func (s *Session) finalizeIdentity() error {
	if len(s.icState.violations) > 0 {
		return s.icState.violations[0]
	}
	if len(s.icState.completed) == 0 {
		return nil
	}
	constraints := make([]ic.Constraint, 0, len(s.icState.completed))
	for _, scope := range s.icState.completed {
		for _, constraint := range scope.constraints {
			rows := make([]ic.Row, 0, len(constraint.rows))
			for _, row := range constraint.rows {
				rows = append(rows, ic.Row{Values: row.values, Hash: row.hash})
			}
			keyrefs := make([]ic.Row, 0, len(constraint.keyrefs))
			for _, row := range constraint.keyrefs {
				keyrefs = append(keyrefs, ic.Row{Values: row.values, Hash: row.hash})
			}
			constraints = append(constraints, ic.Constraint{
				ID:         constraint.id,
				Category:   constraint.category,
				Referenced: constraint.referenced,
				Rows:       rows,
				Keyrefs:    keyrefs,
			})
		}
	}
	issues := ic.Resolve(constraints)
	if len(issues) == 0 {
		return nil
	}
	issue := issues[0]
	switch issue.Kind {
	case ic.IssueDuplicate:
		return newValidationError(xsderrors.ErrIdentityDuplicate, "identity constraint duplicate")
	case ic.IssueKeyrefMissing:
		return newValidationError(xsderrors.ErrIdentityKeyRefFailed, "identity constraint keyref missing")
	case ic.IssueKeyrefUndefined:
		return newValidationError(xsderrors.ErrIdentityAbsent, "identity constraint keyref undefined")
	default:
		return newValidationError(xsderrors.ErrIdentityAbsent, "identity constraint violation")
	}
}
