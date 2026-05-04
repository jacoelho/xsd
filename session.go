package xsd

import (
	"encoding/xml"
	"errors"
	"io"
)

// Validate validates one XML instance document.
func (e *Engine) Validate(r io.Reader) error {
	return (&session{engine: e}).validate(r)
}

type session struct {
	schemaLocationNamespaces map[string]bool
	ids                      map[string]string
	engine                   *Engine
	ns                       namespaceStack
	idrefs                   []identityRef
	namePath                 []runtimeName
	stack                    []frame
	elementNames             []xml.Name
	path                     []string
	counters                 []uint32
	idScopes                 []identityScope
	idSelections             []identitySelection
	text                     []byte
	counterScratch           []uint32
	nameStrings              byteStringCache
	valueStrings             byteStringCache
}

type identityRef struct {
	Value string
	Path  string
	Line  int
	Col   int
}

type identityScope struct {
	Tables      map[identityConstraintID]map[string]string
	Constraints []identityConstraintID
	Refs        []identityTupleRef
	Depth       int
}

type identityTupleRef struct {
	Key        string
	Path       string
	Line       int
	Col        int
	Constraint identityConstraintID
	Refer      identityConstraintID
}

const identityConflictPath = "\x00identity-conflict"

type identitySelection struct {
	Path       string
	Values     []string
	Present    []bool
	Scope      int
	Depth      int
	Line       int
	Col        int
	Constraint identityConstraintID
}

type frame struct {
	Children    []runtimeName
	Index       int
	CounterBase int
	CounterLen  int
	Choice      int
	TextStart   int
	Type        typeID
	Model       contentModelID
	Element     elementID
	Nilled      bool
	Skip        bool
	HasChild    bool
	HasText     bool
}

type namespaceBinding struct {
	Prefix string
	URI    string
}

type namespaceStack struct {
	frames   []int
	bindings []namespaceBinding
}

func (s *session) validate(r io.Reader) error {
	if s == nil || s.engine == nil || s.engine.rt == nil {
		return &Error{Category: InternalErrorCategory, Code: ErrInternalInvariant, Message: "nil validation session"}
	}
	s.reset()
	reader, err := prepareInstanceReader(r)
	if err != nil {
		return err
	}
	parser := newXMLStreamParser(reader, &s.nameStrings, &s.valueStrings)
	seenRoot := false
	for {
		tok, err := parser.next()
		if err == io.EOF {
			break
		}
		line, col := tok.line, tok.col
		if err != nil {
			if errors.Is(err, errUnsupportedEntityReference) {
				return &Error{Category: UnsupportedErrorCategory, Code: ErrUnsupportedExternal, Line: line, Column: col, Path: s.pathString(), Message: "external or undeclared entity resolution is not supported", Err: err}
			}
			return validation(ErrValidationXML, line, col, s.pathString(), err.Error())
		}
		switch tok.kind {
		case streamTokenStart:
			if err := s.start(tok.line, tok.col, tok.start, seenRoot); err != nil {
				return err
			}
			seenRoot = true
		case streamTokenEnd:
			if err := s.end(tok.line, tok.col, tok.end); err != nil {
				return err
			}
		case streamTokenCharData:
			if err := s.chars(tok.line, tok.col, tok.data, tok.cdata); err != nil {
				return err
			}
		case streamTokenDirective:
			if isDOCTYPEDeclaration(tok.directive) {
				return &Error{Category: UnsupportedErrorCategory, Code: ErrUnsupportedDTD, Line: line, Column: col, Path: s.pathString(), Message: "DTD declarations are not supported"}
			}
		}
	}
	if !seenRoot {
		return validation(ErrValidationRoot, 0, 0, "", "instance document has no root element")
	}
	if len(s.stack) != 0 {
		return validation(ErrValidationXML, 0, 0, s.pathString(), "unclosed element")
	}
	if err := s.checkIDRefs(); err != nil {
		return err
	}
	return nil
}

func (s *session) reset() {
	s.stack = s.stack[:0]
	s.ns.frames = s.ns.frames[:0]
	s.ns.bindings = s.ns.bindings[:0]
	s.text = s.text[:0]
	s.path = s.path[:0]
	s.namePath = s.namePath[:0]
	s.elementNames = s.elementNames[:0]
	s.counters = s.counters[:0]
	for k := range s.ids {
		delete(s.ids, k)
	}
	s.idrefs = s.idrefs[:0]
	s.idScopes = s.idScopes[:0]
	s.idSelections = s.idSelections[:0]
	for ns := range s.schemaLocationNamespaces {
		delete(s.schemaLocationNamespaces, ns)
	}
	s.counterScratch = s.counterScratch[:0]
}

func (s *session) start(line, col int, se xml.StartElement, seenRoot bool) error {
	rt := s.engine.rt
	if err := s.ns.push(se.Attr); err != nil {
		return validation(ErrValidationXML, line, col, s.pathString(), err.Error())
	}
	var err error
	se, err = s.translateStartElement(se, line, col)
	if err != nil {
		return err
	}
	s.recordSchemaLocationHints(se.Attr)
	rn := s.runtimeName(se.Name)
	var elem elementID
	var typ typeID
	hasType := false
	skip := false
	if !seenRoot {
		if rn.Known {
			if id, ok := rt.GlobalElements[rn.Name]; ok {
				elem = id
				typ = rt.Elements[id].Type
				hasType = true
			}
		}
		if !hasType {
			rootType, ok, err := s.rootTypeFromXSIType(se.Attr, line, col)
			if err != nil {
				return err
			}
			if !ok {
				if s.hasSchemaLocationHint(rn.NS) {
					return s.unsupportedSchemaLocation(line, col, "element", rn)
				}
				return validation(ErrValidationRoot, line, col, s.pathString(), "root element is not declared: "+formatXMLName(se.Name))
			}
			elem = noElement
			typ = rootType
		}
	} else {
		if len(s.stack) == 0 {
			return validation(ErrValidationXML, line, col, s.pathString(), "multiple root elements")
		}
		parent := &s.stack[len(s.stack)-1]
		parent.HasChild = true
		accepted, err := s.acceptChild(parent, rn, se.Attr, line, col)
		if err != nil {
			return err
		}
		elem = accepted.element
		typ = accepted.typ
		skip = accepted.skip
	}
	if elem != noElement && rt.Elements[elem].Abstract {
		return validation(ErrValidationElement, line, col, s.pathString(), "abstract element cannot appear directly")
	}
	nilled := false
	if !skip {
		var err error
		typ, nilled, err = s.effectiveType(elem, typ, se.Attr, line, col)
		if err != nil {
			return err
		}
	}
	s.pushFrame(elem, typ, nilled, skip)
	s.path = append(s.path, rn.Local)
	s.namePath = append(s.namePath, rn)
	s.elementNames = append(s.elementNames, se.Name)
	s.startIdentityScope(elem)
	s.matchIdentitySelectors(line, col)
	if !skip {
		if err := s.validateAttributes(typ, se.Attr, line, col); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) pushFrame(elem elementID, typ typeID, nilled, skip bool) {
	rt := s.engine.rt
	modelID := noContentModel
	counterLen := 0
	if typ.Kind == typeComplex {
		ct := rt.ComplexTypes[typ.ID]
		modelID = ct.Content
		if modelID != noContentModel {
			counterLen = modelCounterLen(rt, rt.Models[modelID])
		}
	}
	base := len(s.counters)
	for i := 0; i < counterLen; i++ {
		s.counters = append(s.counters, 0)
	}
	s.stack = append(s.stack, frame{
		Element:     elem,
		Type:        typ,
		Model:       modelID,
		CounterBase: base,
		CounterLen:  counterLen,
		TextStart:   len(s.text),
		Nilled:      nilled,
		Skip:        skip,
		Choice:      -1,
	})
}

func (s *session) chars(line, col int, data []byte, cdata bool) error {
	if len(s.stack) == 0 {
		if cdata {
			return validation(ErrValidationXML, line, col, s.pathString(), "CDATA section outside root element")
		}
		if isXMLWhitespaceBytes(data) {
			return nil
		}
		return validation(ErrValidationText, line, col, s.pathString(), "text outside root element")
	}
	f := &s.stack[len(s.stack)-1]
	if len(data) == 0 {
		return nil
	}
	if f.Skip {
		return nil
	}
	if f.Nilled {
		return validation(ErrValidationNil, line, col, s.pathString(), "nilled element must be empty")
	}
	if !isXMLWhitespaceBytes(data) {
		f.HasText = true
	}
	if f.Type.Kind == typeSimple || (f.Type.Kind == typeComplex && s.engine.rt.ComplexTypes[f.Type.ID].SimpleValue) {
		s.text = append(s.text, data...)
		return nil
	}
	if f.Type.Kind == typeComplex {
		ct := s.engine.rt.ComplexTypes[f.Type.ID]
		if ct.Mixed {
			if f.Element != noElement && s.engine.rt.Elements[f.Element].HasFixed {
				s.text = append(s.text, data...)
			}
			return nil
		}
		if !isXMLWhitespaceBytes(data) {
			return validation(ErrValidationText, line, col, s.pathString(), "character data is not allowed")
		}
	}
	return nil
}

func isXMLWhitespaceBytes(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}
