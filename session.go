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

// NewSession creates a reusable validation session. Reused sessions retain
// bounded scratch buffers and string caches; create a new session to release
// retained cache contents.
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

// Validate validates one XML instance document and resets validation state
// first. It may retain bounded scratch buffers and string caches for reuse.
func (s *Session) Validate(r io.Reader) error {
	if s == nil {
		return (*session)(nil).validate(r)
	}
	return s.session.validate(r)
}

// Reset clears validation state while preserving options. It may retain bounded
// scratch buffers and string caches; create a new session to release retained
// cache contents.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.session.reset()
}

// session holds the state for validating documents against one Engine.
// Per-document state lives in doc; everything else is retained across
// documents: options, the reader and parser, and the string and path caches.
type session struct {
	engine                *Engine
	doc                   documentState
	pathCache             map[pathCacheKey]string
	nameStrings           byteStringCache
	valueStrings          byteStringCache
	reader                *bufio.Reader
	parser                xmlStreamParser
	maxErrors             int
	maxIdentityScopes     int
	maxIdentityEntries    int
	maxIdentityTupleBytes int64
	maxInstanceDepth      int
	maxInstanceAttributes int
	maxInstanceTextBytes  int64
	maxInstanceTokenBytes int64
}

// documentState is the mutable state of one document validation. The token
// loop, frame stack, and error accumulation live in session.go; content-model
// state (stack frames, allBits) is driven by session_model.go; identity
// constraint state (ids, idrefs, idScopes, idSelections, identityFieldValues,
// identityMatches, identityEntries) by session_identity.go; attribute
// validation by session_attributes.go; namespace and xsi handling (ns,
// schemaLocationNamespaces) by session_namespaces.go; reader and parser setup
// by session_reader.go.
//
// session.reset rebuilds the whole struct with a composite literal, so any
// field not named there is zeroed between documents; listing a field in
// reset only opts it into capacity reuse, never into surviving a reset.
type documentState struct {
	ids                      map[string]string
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
	identityFieldValues      []identityFieldValue
	identityMatches          []identityFieldMatch
	text                     []byte
	identityEntries          int
	pathTextDepth            int
}

type pathCacheKey struct {
	Parent string
	Local  string
}

type identityRef struct {
	Value string
	Path  string
	Line  int
	Col   int
}

type identityScope struct {
	Tables      map[identityConstraintID]map[string]identityTableEntry
	Constraints []identityConstraintID
	Refs        []identityTupleRef
	Depth       int
}

// identityTableEntry records where a key tuple was first seen. Conflict marks
// tuples that propagated from child scopes with differing paths; conflicted
// tuples cannot resolve keyrefs.
type identityTableEntry struct {
	Path     string
	Conflict bool
}

type identityTupleRef struct {
	Key        string
	Path       string
	Line       int
	Col        int
	Constraint identityConstraintID
	Refer      identityConstraintID
}

type identitySelection struct {
	Path       string
	Scope      int
	Depth      int
	FieldStart int
	FieldLen   int
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
	s.parser.lazyAttrValue = true
	s.parser.maxAttrs = s.maxInstanceAttributes
	seenRoot := false
	for {
		tok, err := s.parser.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return s.parseError(tok, err)
		}
		switch tok.kind {
		case streamTokenStart:
			if err := s.start(tok.line, tok.col, tok.start, seenRoot); err != nil {
				return s.stopOrError(err)
			}
			seenRoot = true
		case streamTokenEnd:
			if err := s.end(tok.line, tok.col, tok.end); err != nil {
				return s.stopOrError(err)
			}
		case streamTokenCharData:
			if err := s.chars(tok.line, tok.col, tok.data, tok.cdata); err != nil {
				if recoverErr := s.recover(err); recoverErr != nil {
					return s.stopOrError(recoverErr)
				}
			}
		case streamTokenDirective:
			if isDOCTYPEDeclaration(tok.directive) {
				return &Error{Category: UnsupportedErrorCategory, Code: ErrUnsupportedDTD, Line: tok.line, Column: tok.col, Path: s.pathString(), Message: "DTD declarations are not supported"}
			}
		case streamTokenComment, streamTokenPI:
		}
	}
	return s.finishValidation(seenRoot)
}

func (s *session) parseError(tok streamToken, err error) error {
	line, col := tok.line, tok.col
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

func (s *session) stopOrError(err error) error {
	if errors.Is(err, errStopValidation) {
		return s.result()
	}
	return err
}

func (s *session) finishValidation(seenRoot bool) error {
	if !seenRoot {
		return validation(ErrValidationRoot, 0, 0, "", "instance document has no root element")
	}
	if len(s.doc.stack) != 0 {
		return validation(ErrValidationXML, 0, 0, s.pathString(), "unclosed element")
	}
	if err := s.checkIDRefs(); err != nil {
		return s.stopOrError(err)
	}
	return s.result()
}

// reset rebuilds the per-document state. Fields named in the literal recycle
// bounded capacity from the previous document; every other documentState
// field is zeroed by the literal itself, so omitting a field can never leak
// state across documents.
func (s *session) reset() {
	s.doc = documentState{
		errors: resetRetainedSlice(s.doc.errors),
		stack:  resetRetainedSlice(s.doc.stack),
		ns: namespaceStack{
			frames:   resetRetainedSlice(s.doc.ns.frames),
			bindings: resetRetainedSlice(s.doc.ns.bindings),
		},
		text:                     resetRetainedBytes(s.doc.text),
		path:                     resetRetainedSlice(s.doc.path),
		namePath:                 resetRetainedSlice(s.doc.namePath),
		elementNames:             resetRetainedSlice(s.doc.elementNames),
		allBits:                  resetRetainedSlice(s.doc.allBits),
		ids:                      resetRetainedMap(s.doc.ids),
		idrefs:                   resetRetainedSlice(s.doc.idrefs),
		idScopes:                 resetRetainedSlice(s.doc.idScopes),
		idSelections:             resetRetainedSlice(s.doc.idSelections),
		identityFieldValues:      resetRetainedSlice(s.doc.identityFieldValues),
		identityMatches:          resetRetainedSlice(s.doc.identityMatches),
		schemaLocationNamespaces: resetRetainedMap(s.doc.schemaLocationNamespaces),
	}
	if len(s.pathCache) > maxRetainedMapLen {
		s.pathCache = nil
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

func resetRetainedMap[K comparable, V any](m map[K]V) map[K]V {
	if len(m) > maxRetainedMapLen {
		return nil
	}
	clear(m)
	return m
}

func (s *session) result() error {
	switch len(s.doc.errors) {
	case 0:
		return nil
	case 1:
		return s.doc.errors[0]
	default:
		return Errors(slices.Clone(s.doc.errors))
	}
}

func (s *session) recover(err error) error {
	if err == nil {
		return nil
	}
	if !isRecoverableValidation(err) {
		return err
	}
	s.doc.errors = append(s.doc.errors, err)
	if s.maxErrors > 0 && len(s.doc.errors) >= s.maxErrors {
		return errStopValidation
	}
	return nil
}

// isRecoverableValidation documents recoverable validation policy.
func isRecoverableValidation(err error) bool {
	x, ok := errors.AsType[*Error](err)
	return ok && x.Category == ValidationErrorCategory && x.Code != ErrValidationXML && x.Code != ErrValidationLimit
}

func (s *session) start(line, col int, se streamStartElement, seenRoot bool) error {
	if s.maxInstanceDepth > 0 && len(s.doc.stack)+1 > s.maxInstanceDepth {
		return validation(ErrValidationLimit, line, col, s.pathString(), "instance depth limit exceeded")
	}
	if s.maxInstanceAttributes > 0 && len(se.Attr) > s.maxInstanceAttributes {
		return validation(ErrValidationLimit, line, col, s.pathString(), "instance attribute limit exceeded")
	}
	rt := s.engine.rt
	if err := s.pushNamespaces(se.Attr); err != nil {
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
	pathName := rn.Local
	if !skip {
		pathName = rn.label()
	}
	s.pushPath(pathName)
	s.doc.namePath = append(s.doc.namePath, rn)
	s.doc.elementNames = append(s.doc.elementNames, se.Name)
	if len(rt.Identities) != 0 {
		if err := s.startIdentityScope(elem, line, col); err != nil {
			return err
		}
		s.matchIdentitySelectors(line, col)
	}
	if !skip {
		if err := s.validateAttributes(typ, se.Attr, line, col); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) startType(rt *runtimeSchema, rn runtimeName, se streamStartElement, line, col int, seenRoot bool) (elementID, typeID, bool, error) {
	if !seenRoot {
		return s.rootStartType(rt, rn, se, line, col)
	}
	if len(s.doc.stack) == 0 {
		return noElement, typeID{}, false, validation(ErrValidationXML, line, col, s.pathString(), "multiple root elements")
	}
	parent := &s.doc.stack[len(s.doc.stack)-1]
	parent.HasChild = true
	accepted, err := s.acceptChild(parent, rn, se.Attr, line, col)
	if err == nil {
		return accepted.element, accepted.typ, accepted.skip, nil
	}
	recoverErr := s.recover(err)
	if recoverErr != nil {
		return noElement, typeID{}, false, recoverErr
	}
	accepted = skippedAnyTypeChild(rt)
	return accepted.element, accepted.typ, accepted.skip, nil
}

func (s *session) rootStartType(rt *runtimeSchema, rn runtimeName, se streamStartElement, line, col int) (elementID, typeID, bool, error) {
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
		return noElement, typeID{}, false, s.unsupportedSchemaLocation(line, col, xsdElemElement, rn)
	}
	err = validation(ErrValidationRoot, line, col, s.pathString(), "root element is not declared: "+formatXMLName(se.Name))
	if recoverErr := s.recover(err); recoverErr != nil {
		return noElement, typeID{}, false, recoverErr
	}
	return noElement, anyType(rt), true, nil
}

func anyType(rt *runtimeSchema) typeID {
	return complexRef(rt.Builtin.AnyType)
}

func (s *session) pushFrame(elem elementID, typ typeID, nilled, skip bool) {
	rt := s.engine.rt
	modelID := noContentModel
	bitLen := 0
	state := uint32(0)
	if id, ok := typ.complex(); ok {
		ct := rt.ComplexTypes[id]
		modelID = ct.Content
		if modelID != noContentModel {
			model := rt.CompiledModels[modelID]
			bitLen = int(model.AllBitLen)
			state = model.Start
		}
	}
	bitBase := len(s.doc.allBits)
	if bitLen > 0 {
		s.doc.allBits = slices.Grow(s.doc.allBits, bitLen)
		s.doc.allBits = s.doc.allBits[:bitBase+bitLen]
		clear(s.doc.allBits[bitBase:])
	}
	s.doc.stack = append(s.doc.stack, frame{
		Element:   elem,
		Type:      typ,
		Model:     modelID,
		BitBase:   bitBase,
		BitLen:    bitLen,
		State:     state,
		TextStart: len(s.doc.text),
		Nilled:    nilled,
		Skip:      skip,
	})
}

func (s *session) chars(line, col int, data []byte, cdata bool) error {
	if len(s.doc.stack) == 0 {
		if cdata {
			return validation(ErrValidationXML, line, col, s.pathString(), "CDATA section outside root element")
		}
		if isXMLWhitespaceBytes(data) {
			return nil
		}
		return validation(ErrValidationText, line, col, s.pathString(), "text outside root element")
	}
	f := &s.doc.stack[len(s.doc.stack)-1]
	if len(data) == 0 {
		return nil
	}
	if f.Skip {
		return nil
	}
	if f.Nilled {
		return validation(ErrValidationNil, line, col, s.pathString(), "nilled element must be empty")
	}
	if s.engine.rt.typeHasSimpleContent(f.Type) {
		return s.appendText(data, line, col)
	}
	whitespace := isXMLWhitespaceBytes(data)
	if !whitespace {
		f.HasText = true
	}
	if id, ok := f.Type.complex(); ok {
		ct := s.engine.rt.ComplexTypes[id]
		if ct.mixed() {
			if f.Element != noElement && s.engine.rt.Elements[f.Element].Fixed.Present {
				if err := s.appendText(data, line, col); err != nil {
					return err
				}
			}
			return nil
		}
		if !whitespace {
			return validation(ErrValidationText, line, col, s.pathString(), "character data is not allowed")
		}
	}
	return nil
}

func (s *session) appendText(data []byte, line, col int) error {
	if s.maxInstanceTextBytes > 0 && int64(len(s.doc.text)+len(data)) > s.maxInstanceTextBytes {
		return validation(ErrValidationLimit, line, col, s.pathString(), "instance text byte limit exceeded")
	}
	s.doc.text = append(s.doc.text, data...)
	return nil
}
