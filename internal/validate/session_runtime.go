package validate

import (
	"bufio"
	"errors"
	"io"
	"slices"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

var errStopValidation = errors.New("validation stopped after maximum errors")

const (
	maxRetainedSliceCap  = 4096
	maxRetainedBufferCap = 1 << 20
	maxRetainedMapLen    = 4096
)

// Session validates XML instance documents against one compiled runtime.
//
// A Session is not goroutine-safe. Use separate sessions for concurrent validation.
type Session struct {
	session session
}

// NewSession creates a reusable validation session.
func NewSession(rt *runtime.Schema, opts Options) (Session, error) {
	var inner session
	if err := initializeSession(&inner, rt, opts); err != nil {
		return Session{}, err
	}
	return Session{session: inner}, nil
}

func initializeSession(s *session, rt *runtime.Schema, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	hasIdentityConstraints := rt != nil && rt.HasIdentityConstraints()
	*s = session{
		rt:                     rt,
		hasIdentityConstraints: hasIdentityConstraints,
		maxErrors:              limits.Errors,
		maxIdentityScopes:      limits.IdentityScopes,
		maxIdentityEntries:     limits.IdentityEntries,
		maxIdentityTupleBytes:  limits.IdentityTupleBytes,
		maxInstanceDepth:       limits.InstanceDepth,
		maxInstanceAttributes:  limits.InstanceAttributes,
		maxInstanceTextBytes:   limits.InstanceTextBytes,
		maxInstanceTokenBytes:  limits.InstanceTokenBytes,
	}
	return nil
}

// Validate validates one XML instance document with isolated per-call state.
func Validate(rt *runtime.Schema, r io.Reader, opts Options) error {
	var s session
	if err := initializeSession(&s, rt, opts); err != nil {
		return err
	}
	return s.validate(r)
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
// documents: options, the reader buffer and parser, and the string caches.
type session struct {
	rt                            *runtime.Schema
	resolveLexicalQNamePartsFunc  runtime.ResolveQNameParts
	resolveLexicalQNamePartsOwner *session
	reader                        *bufio.Reader
	doc                           documentState
	nameStrings                   stream.Cache
	valueStrings                  stream.Cache
	attributeSeen                 []bool
	parser                        stream.Parser
	maxErrors                     int
	maxIdentityScopes             int
	maxIdentityEntries            int
	maxIdentityTupleBytes         int64
	maxInstanceDepth              int
	maxInstanceAttributes         int
	maxInstanceTextBytes          int64
	maxInstanceTokenBytes         int64
	hasIdentityConstraints        bool
}

// documentState is the mutable state of one document validation. XML syntax
// and content-model frames live in one xmlDocument stack; identity constraint
// selector matching is owned by session_identity.go; attribute validation by
// session_attributes.go.
//
// session.reset rebuilds the whole struct with a composite literal, so any
// field not named there is zeroed between documents; listing a field in
// reset only opts it into capacity reuse, never into surviving a reset.
//
//nolint:govet // Field order groups retained validation state by owning subsystem.
type documentState struct {
	xmlDocument[frame]
	identity            IdentityState
	schemaLocationHints SchemaLocationHints
	allBits             []uint64
	namePath            []runtime.RuntimeName
	errors              []error
	text                []byte
}

type frame struct {
	Index                     int
	BitBase                   int
	BitLen                    int
	TextStart                 int
	Content                   runtime.ContentState
	Child                     runtime.ChildContentInfo
	Type                      runtime.TypeID
	SimpleContent             runtime.SimpleTypeID
	Element                   runtime.ElementID
	Nilled                    bool
	Skip                      bool
	SimpleContentKnown        bool
	HasSimpleContent          bool
	HasChild                  bool
	HasText                   bool
	ChildOK                   bool
	ElementValueKnown         bool
	ElementDeclared           bool
	ElementHasValueConstraint bool
}

func (s *session) validate(r io.Reader) error {
	if s == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	defer s.detachReader()
	if s.rt == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	s.reset()
	reader, err := PrepareInstanceReaderWithBuffer(r, s.reader)
	if err != nil {
		return err
	}
	s.reader = reader
	s.parser.ResetWithLimit(reader, &s.nameStrings, &s.valueStrings, s.maxInstanceTokenBytes)
	s.parser.SetLazyAttrValue(true)
	s.parser.SetMaxAttrs(s.maxInstanceAttributes)
	for {
		tok, err := s.parser.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return s.parseError(tok, err)
		}
		switch tok.Kind {
		case stream.KindStart:
			if err := s.start(tok.Line, tok.Column, tok.Start); err != nil {
				return s.stopOrError(err)
			}
		case stream.KindEnd:
			if err := s.end(tok.Line, tok.Column, tok.End); err != nil {
				return s.stopOrError(err)
			}
		case stream.KindCharData:
			if err := s.chars(tok.Line, tok.Column, tok.Data, tok.CDATA); err != nil {
				if recoverErr := s.recover(err); recoverErr != nil {
					return s.stopOrError(recoverErr)
				}
			}
		case stream.KindDirective:
			return ValidateDirective(s.startContext(tok.Line, tok.Column), tok.Directive)
		case stream.KindComment, stream.KindPI:
		}
	}
	return s.finishValidation()
}

func (s *session) parseError(tok stream.Token, err error) error {
	line, col := tok.Line, tok.Column
	if line == 0 {
		line, col = s.parser.Pos()
	}
	return StreamError(line, col, s.doc.PathString(), err)
}

func (s *session) stopOrError(err error) error {
	if errors.Is(err, errStopValidation) {
		return s.result()
	}
	return err
}

func (s *session) finishValidation() error {
	if err := s.doc.Complete(); err != nil {
		return err
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
	s.detachReader()
	xmlDocument := s.doc.xmlDocument
	xmlDocument.Reset(maxRetainedSliceCap)
	schemaLocationHints := s.doc.schemaLocationHints
	schemaLocationHints.Reset(maxRetainedMapLen)
	identity := s.doc.identity
	identity.Reset(maxRetainedMapLen, maxRetainedSliceCap)
	s.doc = documentState{
		xmlDocument:         xmlDocument,
		errors:              resetRetainedReferences(s.doc.errors, maxRetainedSliceCap),
		text:                resetRetainedBytes(s.doc.text),
		namePath:            resetRetainedReferences(s.doc.namePath, maxRetainedSliceCap),
		allBits:             resetRetainedValues(s.doc.allBits, maxRetainedSliceCap),
		identity:            identity,
		schemaLocationHints: schemaLocationHints,
	}
}

func (s *session) detachReader() {
	if s.reader != nil {
		s.reader.Reset(nil)
	}
}

func resetRetainedReferences[T any](s []T, maxRetainedCap int) []T {
	if cap(s) > maxRetainedCap {
		return nil
	}
	clear(s)
	return s[:0]
}

func resetRetainedValues[T any](s []T, maxRetainedCap int) []T {
	if cap(s) > maxRetainedCap {
		return nil
	}
	return s[:0]
}

func resetRetainedBytes(s []byte) []byte {
	if cap(s) > maxRetainedBufferCap {
		return nil
	}
	return s[:0]
}

func (s *session) result() error {
	switch len(s.doc.errors) {
	case 0:
		return nil
	case 1:
		return s.doc.errors[0]
	default:
		return xsderrors.Errors(slices.Clone(s.doc.errors))
	}
}

func (s *session) recover(err error) error {
	if err == nil {
		return nil
	}
	if !RecoverableError(err) {
		return err
	}
	s.doc.errors = append(s.doc.errors, err)
	if RecoveryLimitReached(len(s.doc.errors), s.maxErrors) {
		return errStopValidation
	}
	return nil
}

func (s *session) start(line, col int, se stream.StartElement) error {
	se, err := s.doc.PrepareStart(se, &s.valueStrings, s.maxInstanceDepth, line, col)
	if err != nil {
		return err
	}
	xsiFlags := xsiStartAttributeFlagsFor(se.Attr)
	if xsiFlags.SchemaLocation {
		if schemaLocationErr := s.recordSchemaLocationHints(se.Attr, line, col); schemaLocationErr != nil {
			recoverErr := s.recover(schemaLocationErr)
			if recoverErr != nil {
				s.doc.AbortStart()
				return recoverErr
			}
		}
	}
	rn := s.runtimeName(se.Name)
	start, err := s.startType(rn, se, xsiFlags.Type, line, col)
	if err != nil {
		s.doc.AbortStart()
		return err
	}
	var nilled bool
	var decl runtime.ElementStartInfo
	var declared bool
	if !start.skip {
		ctx := s.startContext(line, col)
		decl, declared = s.rt.Element(start.element)
		nilled, err = s.assessElementStart(&start, decl, declared, se.Attr, xsiFlags, ctx)
		if err != nil {
			if recoverErr := s.recover(err); recoverErr != nil {
				s.doc.AbortStart()
				return recoverErr
			}
		}
		if start.skip {
			decl = runtime.ElementStartInfo{}
			declared = false
		}
	}
	schemaFrame := s.newSchemaFrame(
		start.element,
		start.typ,
		nilled,
		start.skip,
		!start.skip,
		declared,
		declared && (decl.Fixed || decl.Default),
	)
	s.doc.CommitStart(se.Name, se.RawName, !start.skip && !rn.Known && rn.NS != "", schemaFrame)
	if s.hasIdentityConstraints {
		s.doc.namePath = append(s.doc.namePath, rn)
		if scopeErr := s.startIdentityScope(start.element, line, col); scopeErr != nil {
			return scopeErr
		}
		if matchErr := s.matchIdentitySelectors(line, col); matchErr != nil {
			return matchErr
		}
	}
	if !start.skip {
		if attrErr := s.validateAttributes(start.typ, se.Attr, line, col); attrErr != nil {
			return attrErr
		}
	}
	return nil
}

func (s *session) assessElementStart(
	start *schemaStart,
	decl runtime.ElementStartInfo,
	declared bool,
	attrs []stream.Attr,
	flags xsiStartAttributeFlags,
	ctx StartContext,
) (bool, error) {
	if declared && decl.Abstract {
		*start = schemaStart{element: runtime.NoElement, typ: s.rt.AnyType(), skip: true}
		return false, validation(ctx, xsderrors.CodeValidationElement, "abstract element cannot appear directly")
	}

	nilled := false
	nilSpecified := false
	if flags.Type || flags.Nil {
		for i := range attrs {
			a := &attrs[i]
			if a.Name.Space != vocab.XSINamespaceURI {
				continue
			}
			value := a.StringValue(&s.valueStrings)
			switch a.Name.Local {
			case vocab.XSIAttrNil:
				nilSpecified = true
				parsed, ok := ParseXSINil(value)
				if !ok {
					return false, validation(ctx, xsderrors.CodeValidationNil, "invalid xsi:nil value")
				}
				nilled = parsed
			case vocab.XSIAttrType:
				override, err := resolveXSIType(s.rt, value, s.qnameResolver(), s.schemaLocationHintLookup(), ctx)
				if err != nil {
					return nilled, err
				}
				if err := validateXSITypeOverride(s.rt, start.typ, override, decl.Block, declared, ctx); err != nil {
					return nilled, err
				}
				start.typ = override
			}
		}
	}

	var info runtime.TypeInfo
	infoKnown := true
	if start.typ.Kind == runtime.TypeComplex {
		info, infoKnown = s.rt.TypeInfo(start.typ)
	}
	var err error
	start.typ, nilled, err = validateElementEffectiveState(
		decl, declared, start.typ, nilled, nilSpecified, info, infoKnown, ctx,
	)
	return nilled, err
}

type schemaStart struct {
	element runtime.ElementID
	typ     runtime.TypeID
	skip    bool
}

func (s *session) startType(rn runtime.RuntimeName, se stream.StartElement, hasXSIType bool, line, col int) (schemaStart, error) {
	if s.doc.Depth() == 0 {
		return s.rootStartType(rn, se, hasXSIType, line, col)
	}
	parent, ok := s.doc.Current()
	if !ok {
		return schemaStart{}, xsderrors.InternalInvariant("child start has no parent frame")
	}
	parent.HasChild = true
	accepted, err := s.acceptChild(parent, rn, hasXSIType, line, col)
	if err == nil {
		return schemaStart{
			element: accepted.element,
			typ:     accepted.typ,
			skip:    accepted.skip,
		}, nil
	}
	if !accepted.recover {
		return schemaStart{}, err
	}
	recoverErr := s.recover(err)
	if recoverErr != nil {
		return schemaStart{}, recoverErr
	}
	return schemaStart{
		element: accepted.element,
		typ:     accepted.typ,
		skip:    accepted.skip,
	}, nil
}

func (s *session) rootStartType(rn runtime.RuntimeName, se stream.StartElement, hasXSIType bool, line, col int) (schemaStart, error) {
	input := RootInput{
		Name:              se.Name,
		RuntimeName:       rn,
		Values:            &s.valueStrings,
		ResolveQNameParts: s.qnameResolverForAttrs(hasXSIType),
		HasSchemaLocation: s.schemaLocationHintLookup(),
		Context:           s.startContext(line, col),
	}
	start, err := RootStart(s.rt, se.Attr, input)
	if err != nil {
		if !start.Recover {
			return schemaStart{}, err
		}
		if recoverErr := s.recover(err); recoverErr != nil {
			return schemaStart{}, recoverErr
		}
	}
	return schemaStart{
		element: start.Element,
		typ:     start.Type,
		skip:    start.Skip,
	}, nil
}

func (s *session) startContext(line, col int) StartContext {
	return s.doc.context(line, col)
}

func (s *session) newSchemaFrame(
	elem runtime.ElementID,
	typ runtime.TypeID,
	nilled, skip bool,
	elementValueKnown, elementDeclared, elementHasValueConstraint bool,
) frame {
	contentFrame := s.rt.ContentFrame(typ)
	childContent, childContentOK := s.schemaChildContentInfo(typ)
	if !childContentOK {
		childContent = runtime.ChildContentInfo{}
	}
	simpleContent, hasSimpleContent, simpleContentKnown := s.schemaFrameSimpleContent(typ, childContent, childContentOK)
	bitLen := contentFrame.AllBitLen()
	bitBase := len(s.doc.allBits)
	if bitLen > 0 {
		s.doc.allBits = slices.Grow(s.doc.allBits, bitLen)
		s.doc.allBits = s.doc.allBits[:bitBase+bitLen]
		clear(s.doc.allBits[bitBase:])
	}
	return frame{
		Element:                   elem,
		Type:                      typ,
		BitBase:                   bitBase,
		BitLen:                    bitLen,
		Content:                   contentFrame.ContentState(),
		Child:                     childContent,
		SimpleContent:             simpleContent,
		TextStart:                 len(s.doc.text),
		Nilled:                    nilled,
		Skip:                      skip,
		SimpleContentKnown:        simpleContentKnown,
		HasSimpleContent:          hasSimpleContent,
		ChildOK:                   childContentOK,
		ElementValueKnown:         elementValueKnown,
		ElementDeclared:           elementDeclared,
		ElementHasValueConstraint: elementHasValueConstraint,
	}
}

func (s *session) schemaChildContentInfo(typ runtime.TypeID) (runtime.ChildContentInfo, bool) {
	return s.rt.ChildContent(typ)
}

func (s *session) schemaFrameSimpleContent(
	typ runtime.TypeID,
	child runtime.ChildContentInfo,
	childOK bool,
) (runtime.SimpleTypeID, bool, bool) {
	if id, ok := typ.Simple(); ok {
		return id, true, true
	}
	if !childOK {
		return runtime.NoSimpleType, false, false
	}
	if !child.Simple {
		return runtime.NoSimpleType, false, true
	}
	id, hasSimpleContent, ok := s.rt.SimpleContentType(typ)
	if !ok {
		return runtime.NoSimpleType, false, false
	}
	return id, hasSimpleContent, true
}

func (s *session) chars(line, col int, data []byte, cdata bool) error {
	f, ok := s.doc.Current()
	if !ok {
		return ValidateDocumentCharacterData(data, cdata, s.startContext(line, col))
	}
	if len(data) == 0 || f.Skip {
		return nil
	}
	if f.Nilled {
		return validation(s.startContext(line, col), xsderrors.CodeValidationNil, "nilled element must be empty")
	}
	if frameHasSimpleContent(f) {
		return s.appendText(data, line, col)
	}
	content, ok := s.rt.ElementTextContent(f.Type, f.Element)
	if !ok {
		return xsderrors.InternalInvariant("character data content info is invalid")
	}
	if content.HasSimpleContent() {
		return s.appendText(data, line, col)
	}
	whitespace := lex.IsXMLWhitespaceBytes(data)
	if !whitespace {
		f.HasText = true
	}
	if content.AllowsMixedContent() {
		if content.HasFixedElementValue() {
			return s.appendText(data, line, col)
		}
		return nil
	}
	if content.IsComplexType() && !whitespace {
		ctx := s.startContext(line, col)
		return validation(ctx, xsderrors.CodeValidationText, "character data is not allowed")
	}
	return nil
}

func frameHasSimpleContent(f *frame) bool {
	if f.SimpleContentKnown {
		return f.HasSimpleContent
	}
	return f.Type.Kind == runtime.TypeSimple || (f.ChildOK && f.Child.Simple)
}

func (s *session) appendText(data []byte, line, col int) error {
	if s.maxInstanceTextBytes > 0 && int64(len(s.doc.text)) > s.maxInstanceTextBytes-int64(len(data)) {
		return validation(s.startContext(line, col), xsderrors.CodeValidationLimit, "instance text byte limit exceeded")
	}
	s.doc.text = append(s.doc.text, data...)
	return nil
}
