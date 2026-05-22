package xsd

import (
	"bufio"
	"encoding/xml"
	"errors"
	"io"
	"slices"
)

var errStopValidation = errors.New("validation stopped after maximum errors")

const (
	maxRetainedSliceCap  = 4096
	maxRetainedBufferCap = 1 << 20
	maxRetainedMapLen    = 4096
)

// ValidateOptions controls instance validation.
type ValidateOptions struct {
	// MaxErrors limits collected validation errors. Zero means unlimited.
	MaxErrors int
	// MaxIdentityScopes limits active identity-constraint scopes. Zero means unlimited.
	MaxIdentityScopes int
	// MaxIdentityEntries limits stored ID, IDREF, key, unique, and keyref entries. Zero means unlimited.
	MaxIdentityEntries int
	// MaxIdentityTupleBytes limits the byte length of one stored identity key. Zero means unlimited.
	MaxIdentityTupleBytes int64
	// MaxInstanceDepth limits nested XML elements. Zero means unlimited.
	MaxInstanceDepth int
	// MaxInstanceAttributes limits attributes on one XML element. Zero means unlimited.
	MaxInstanceAttributes int
	// MaxInstanceTextBytes limits retained character data bytes. Zero means unlimited.
	MaxInstanceTextBytes int64
	// MaxInstanceTokenBytes limits retained XML token payload bytes. Zero means unlimited.
	MaxInstanceTokenBytes int64
}

// Session validates XML instance documents against one Engine.
//
// A Session is not goroutine-safe. Use Engine.Validate or separate sessions for
// concurrent validation.
type Session struct {
	session session
}

// Validate validates one XML instance document.
func (e *Engine) Validate(r io.Reader) error {
	return e.ValidateWithOptions(r, ValidateOptions{})
}

// ValidateWithOptions validates one XML instance document with options.
func (e *Engine) ValidateWithOptions(r io.Reader, opts ValidateOptions) error {
	session, err := e.NewSession(opts)
	if err != nil {
		return err
	}
	return session.Validate(r)
}

// NewSession creates a reusable validation session.
func (e *Engine) NewSession(opts ValidateOptions) (*Session, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	return &Session{
		session: session{
			engine:                e,
			maxErrors:             opts.MaxErrors,
			maxIdentityScopes:     opts.MaxIdentityScopes,
			maxIdentityEntries:    opts.MaxIdentityEntries,
			maxIdentityTupleBytes: opts.MaxIdentityTupleBytes,
			maxInstanceDepth:      opts.MaxInstanceDepth,
			maxInstanceAttributes: opts.MaxInstanceAttributes,
			maxInstanceTextBytes:  opts.MaxInstanceTextBytes,
			maxInstanceTokenBytes: opts.MaxInstanceTokenBytes,
		},
	}, nil
}

func validateOptions(opts ValidateOptions) error {
	if opts.MaxErrors < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxErrors cannot be negative")
	}
	if opts.MaxIdentityScopes < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxIdentityScopes cannot be negative")
	}
	if opts.MaxIdentityEntries < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxIdentityEntries cannot be negative")
	}
	if opts.MaxIdentityTupleBytes < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxIdentityTupleBytes cannot be negative")
	}
	if opts.MaxInstanceDepth < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxInstanceDepth cannot be negative")
	}
	if opts.MaxInstanceAttributes < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxInstanceAttributes cannot be negative")
	}
	if opts.MaxInstanceTextBytes < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxInstanceTextBytes cannot be negative")
	}
	if opts.MaxInstanceTokenBytes < 0 {
		return validation(ErrValidationOption, 0, 0, "", "MaxInstanceTokenBytes cannot be negative")
	}
	return nil
}

// Validate validates one XML instance document and resets document-local state first.
func (s *Session) Validate(r io.Reader) error {
	if s == nil {
		return (*session)(nil).validate(r)
	}
	return s.session.validate(r)
}

// Reset clears document-local validation state while preserving options.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.session.reset()
}

type session struct {
	ids                      map[string]string
	engine                   *Engine
	schemaLocationNamespaces map[string]bool
	ns                       namespaceStack
	stack                    []frame
	allBits                  []uint64
	namePath                 []runtimeName
	errors                   []error
	elementNames             []xml.Name
	path                     []string
	pathText                 string
	idrefs                   []identityRef
	idScopes                 []identityScope
	idSelections             []identitySelection
	identityMatches          []identityFieldMatch
	text                     []byte
	nameStrings              byteStringCache
	valueStrings             byteStringCache
	reader                   *bufio.Reader
	parser                   xmlStreamParser
	maxErrors                int
	maxIdentityScopes        int
	maxIdentityEntries       int
	maxIdentityTupleBytes    int64
	maxInstanceDepth         int
	maxInstanceAttributes    int
	maxInstanceTextBytes     int64
	maxInstanceTokenBytes    int64
	identityEntries          int
	pathDirty                bool
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
	Fields     []identityFieldValue
	Scope      int
	Depth      int
	Line       int
	Col        int
	Constraint identityConstraintID
}

type identityFieldMatch struct {
	Selection int
	Field     int
}

type identityFieldValue struct {
	Value   string
	Present bool
}

type frame struct {
	Index     int
	BitBase   int
	BitLen    int
	TextStart int
	State     uint32
	Count     uint32
	Type      typeID
	Model     contentModelID
	Element   elementID
	Nilled    bool
	Skip      bool
	HasChild  bool
	HasText   bool
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
	reader, err := prepareInstanceReaderWithBuffer(r, s.reader)
	if err != nil {
		return err
	}
	s.reader = reader
	s.parser.resetWithLimit(reader, &s.nameStrings, &s.valueStrings, s.maxInstanceTokenBytes)
	s.parser.maxAttrs = s.maxInstanceAttributes
	seenRoot := false
	for {
		tok, err := s.parser.next()
		if err == io.EOF {
			break
		}
		line, col := tok.line, tok.col
		if err != nil {
			if line == 0 {
				line, col = s.parser.br.pos()
			}
			if errors.Is(err, errXMLTokenLimit) || errors.Is(err, errXMLAttributeLimit) {
				return validation(ErrValidationLimit, line, col, s.pathString(), err.Error())
			}
			if errors.Is(err, errUnsupportedEntityReference) {
				return &Error{Category: UnsupportedErrorCategory, Code: ErrUnsupportedExternal, Line: line, Column: col, Path: s.pathString(), Message: "external or undeclared entity resolution is not supported", Err: err}
			}
			return validation(ErrValidationXML, line, col, s.pathString(), err.Error())
		}
		switch tok.kind {
		case streamTokenStart:
			if err := s.start(tok.line, tok.col, tok.start, seenRoot); err != nil {
				if errors.Is(err, errStopValidation) {
					return s.result()
				}
				return err
			}
			seenRoot = true
		case streamTokenEnd:
			if err := s.end(tok.line, tok.col, tok.end); err != nil {
				if errors.Is(err, errStopValidation) {
					return s.result()
				}
				return err
			}
		case streamTokenCharData:
			if err := s.chars(tok.line, tok.col, tok.data, tok.cdata); err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					if errors.Is(recoverErr, errStopValidation) {
						return s.result()
					}
					return recoverErr
				}
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
		if errors.Is(err, errStopValidation) {
			return s.result()
		}
		return err
	}
	return s.result()
}

func (s *session) reset() {
	s.errors = resetRetainedSlice(s.errors)
	s.stack = resetRetainedSlice(s.stack)
	s.ns.frames = resetRetainedSlice(s.ns.frames)
	s.ns.bindings = resetRetainedSlice(s.ns.bindings)
	s.text = resetRetainedBytes(s.text)
	s.path = resetRetainedSlice(s.path)
	s.pathText = ""
	s.pathDirty = true
	s.namePath = resetRetainedSlice(s.namePath)
	s.elementNames = resetRetainedSlice(s.elementNames)
	s.allBits = resetRetainedSlice(s.allBits)
	if len(s.ids) > maxRetainedMapLen {
		s.ids = nil
	} else {
		clear(s.ids)
	}
	s.idrefs = resetRetainedSlice(s.idrefs)
	s.idScopes = resetRetainedSlice(s.idScopes)
	s.idSelections = resetRetainedSlice(s.idSelections)
	s.identityMatches = resetRetainedSlice(s.identityMatches)
	s.identityEntries = 0
	if len(s.schemaLocationNamespaces) > maxRetainedMapLen {
		s.schemaLocationNamespaces = nil
	} else {
		clear(s.schemaLocationNamespaces)
	}
}

func resetRetainedSlice[T any](s []T) []T {
	if cap(s) > maxRetainedSliceCap {
		return nil
	}
	clear(s[:cap(s)])
	return s[:0]
}

func resetRetainedBytes(s []byte) []byte {
	if cap(s) > maxRetainedBufferCap {
		return nil
	}
	return s[:0]
}

func (s *session) result() error {
	switch len(s.errors) {
	case 0:
		return nil
	case 1:
		return s.errors[0]
	default:
		return Errors(slices.Clone(s.errors))
	}
}

func (s *session) recover(err error) error {
	if err == nil {
		return nil
	}
	if !isRecoverableValidation(err) {
		return err
	}
	s.errors = append(s.errors, err)
	if s.maxErrors > 0 && len(s.errors) >= s.maxErrors {
		return errStopValidation
	}
	return nil
}

func isRecoverableValidation(err error) bool {
	x, ok := errors.AsType[*Error](err)
	return ok && x.Category == ValidationErrorCategory && x.Code != ErrValidationXML && x.Code != ErrValidationLimit
}

func (s *session) start(line, col int, se xml.StartElement, seenRoot bool) error {
	if s.maxInstanceDepth > 0 && len(s.stack)+1 > s.maxInstanceDepth {
		return validation(ErrValidationLimit, line, col, s.pathString(), "instance depth limit exceeded")
	}
	if s.maxInstanceAttributes > 0 && len(se.Attr) > s.maxInstanceAttributes {
		return validation(ErrValidationLimit, line, col, s.pathString(), "instance attribute limit exceeded")
	}
	rt := s.engine.rt
	if err := s.ns.push(se.Attr); err != nil {
		return validation(ErrValidationXML, line, col, s.pathString(), err.Error())
	}
	var err error
	se, err = s.translateStartElement(se, line, col)
	if err != nil {
		return err
	}
	if schemaLocationErr := s.recordSchemaLocationHints(se.Attr, line, col); schemaLocationErr != nil {
		recoverErr := s.recover(schemaLocationErr)
		if recoverErr != nil {
			return recoverErr
		}
	}
	rn := s.runtimeName(se.Name)
	elem, typ, skip, err := s.startType(rt, rn, se, line, col, seenRoot)
	if err != nil {
		return err
	}
	if elem != noElement && rt.Elements[elem].Abstract {
		recoverErr := s.recover(validation(ErrValidationElement, line, col, s.pathString(), "abstract element cannot appear directly"))
		if recoverErr != nil {
			return recoverErr
		}
		elem = noElement
		typ = anyType(rt)
		skip = true
	}
	nilled := false
	if !skip {
		var err error
		typ, nilled, err = s.effectiveType(elem, typ, se.Attr, line, col)
		if err != nil {
			recoverErr := s.recover(err)
			if recoverErr != nil {
				return recoverErr
			}
		}
	}
	s.pushFrame(elem, typ, nilled, skip)
	s.path = append(s.path, rn.Local)
	s.pathDirty = true
	s.namePath = append(s.namePath, rn)
	s.elementNames = append(s.elementNames, se.Name)
	if err := s.startIdentityScope(elem, line, col); err != nil {
		return err
	}
	s.matchIdentitySelectors(line, col)
	if !skip {
		if err := s.validateAttributes(typ, se.Attr, line, col); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) startType(rt *runtimeSchema, rn runtimeName, se xml.StartElement, line, col int, seenRoot bool) (elementID, typeID, bool, error) {
	if !seenRoot {
		return s.rootStartType(rt, rn, se, line, col)
	}
	if len(s.stack) == 0 {
		return noElement, typeID{}, false, validation(ErrValidationXML, line, col, s.pathString(), "multiple root elements")
	}
	parent := &s.stack[len(s.stack)-1]
	parent.HasChild = true
	accepted, err := s.acceptChild(parent, rn, se.Attr, line, col)
	if err == nil {
		return accepted.element, accepted.typ, accepted.skip, nil
	}
	recoverErr := s.recover(err)
	if recoverErr != nil {
		return noElement, typeID{}, false, recoverErr
	}
	accepted = anyTypeChild(rt, true)
	return accepted.element, accepted.typ, accepted.skip, nil
}

func (s *session) rootStartType(rt *runtimeSchema, rn runtimeName, se xml.StartElement, line, col int) (elementID, typeID, bool, error) {
	if rn.Known {
		if id, ok := rt.GlobalElements[rn.Name]; ok {
			return id, rt.Elements[id].Type, false, nil
		}
	}
	rootType, ok, err := s.rootTypeFromXSIType(se.Attr, line, col)
	if err != nil {
		return noElement, typeID{}, false, err
	}
	if ok {
		return noElement, rootType, false, nil
	}
	if s.hasSchemaLocationHint(rn.NS) {
		return noElement, typeID{}, false, s.unsupportedSchemaLocation(line, col, "element", rn)
	}
	err = validation(ErrValidationRoot, line, col, s.pathString(), "root element is not declared: "+formatXMLName(se.Name))
	if recoverErr := s.recover(err); recoverErr != nil {
		return noElement, typeID{}, false, recoverErr
	}
	return noElement, anyType(rt), true, nil
}

func anyType(rt *runtimeSchema) typeID {
	return typeID{Kind: typeComplex, ID: uint32(rt.Builtin.AnyType)}
}

func (s *session) pushFrame(elem elementID, typ typeID, nilled, skip bool) {
	rt := s.engine.rt
	modelID := noContentModel
	bitLen := 0
	state := uint32(0)
	if typ.Kind == typeComplex {
		ct := rt.ComplexTypes[typ.ID]
		modelID = ct.Content
		if modelID != noContentModel {
			model := rt.CompiledModels[modelID]
			bitLen = int(model.AllBitLen)
			state = model.Start
		}
	}
	bitBase := len(s.allBits)
	if bitLen > 0 {
		s.allBits = slices.Grow(s.allBits, bitLen)
		s.allBits = s.allBits[:bitBase+bitLen]
		clear(s.allBits[bitBase:])
	}
	s.stack = append(s.stack, frame{
		Element:   elem,
		Type:      typ,
		Model:     modelID,
		BitBase:   bitBase,
		BitLen:    bitLen,
		State:     state,
		TextStart: len(s.text),
		Nilled:    nilled,
		Skip:      skip,
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
		return s.appendText(data, line, col)
	}
	if f.Type.Kind == typeComplex {
		ct := s.engine.rt.ComplexTypes[f.Type.ID]
		if ct.Mixed {
			if f.Element != noElement && s.engine.rt.Elements[f.Element].HasFixed {
				if err := s.appendText(data, line, col); err != nil {
					return err
				}
			}
			return nil
		}
		if !isXMLWhitespaceBytes(data) {
			return validation(ErrValidationText, line, col, s.pathString(), "character data is not allowed")
		}
	}
	return nil
}

func (s *session) appendText(data []byte, line, col int) error {
	if s.maxInstanceTextBytes > 0 && int64(len(s.text)+len(data)) > s.maxInstanceTextBytes {
		return validation(ErrValidationLimit, line, col, s.pathString(), "instance text byte limit exceeded")
	}
	s.text = append(s.text, data...)
	return nil
}
