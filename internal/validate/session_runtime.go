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

// NewSession creates a reusable validation session. Reused sessions retain
// bounded scratch buffers and string caches; create a new session to release
// retained cache contents.
func NewSession(rt Runtime, opts Options) (*Session, error) {
	s := &Session{}
	if err := s.Init(rt, opts); err != nil {
		return nil, err
	}
	return s, nil
}

// PrepareRuntime builds runtime-owned validation hot paths before session use.
func PrepareRuntime(rt Runtime) error {
	schema, ok := rt.(*runtime.Schema)
	if ok {
		return schema.PrepareValidationHotPaths()
	}
	return nil
}

// Init prepares s as a validation session.
func (s *Session) Init(rt Runtime, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	hasIdentityConstraints := false
	var schema *runtime.Schema
	if rt != nil {
		if typed, ok := rt.(*runtime.Schema); ok {
			schema = typed
			if schema == nil {
				rt = nil
			} else if err := PrepareRuntime(schema); err != nil {
				return err
			}
		}
		if rt != nil {
			hasIdentityConstraints = rt.HasIdentityConstraints()
		}
	}
	*s = Session{
		session: session{
			rt:                     rt,
			schema:                 schema,
			hasIdentityConstraints: hasIdentityConstraints,
			maxErrors:              limits.Errors,
			maxIdentityScopes:      limits.IdentityScopes,
			maxIdentityEntries:     limits.IdentityEntries,
			maxIdentityTupleBytes:  limits.IdentityTupleBytes,
			maxInstanceDepth:       limits.InstanceDepth,
			maxInstanceAttributes:  limits.InstanceAttributes,
			maxInstanceTextBytes:   limits.InstanceTextBytes,
			maxInstanceTokenBytes:  limits.InstanceTokenBytes,
		},
	}
	s.session.resolveLexicalQNamePartsFunc = s.session.resolveLexicalQNameParts
	s.session.resolveLexicalQNamePartsOwner = &s.session
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
// documents: options, the reader and parser, and the string caches.
type session struct {
	rt                            Runtime
	schema                        *runtime.Schema
	resolveLexicalQNamePartsFunc  runtime.ResolveQNameParts
	resolveLexicalQNamePartsOwner *session
	reader                        *bufio.Reader
	doc                           documentState
	nameStrings                   stream.Cache
	valueStrings                  stream.Cache
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
// state lives in xmlDocumentState; content-model state (stack frames, allBits)
// is driven by session_model.go; identity constraint selector matching by
// session_identity.go; attribute validation by session_attributes.go.
//
// session.reset rebuilds the whole struct with a composite literal, so any
// field not named there is zeroed between documents; listing a field in
// reset only opts it into capacity reuse, never into surviving a reset.
//
//nolint:govet // Field order groups retained validation state by owning subsystem.
type documentState struct {
	xmlDocumentState
	identity            IdentityState
	schemaLocationHints SchemaLocationHints
	stack               []frame
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
	if s == nil || s.rt == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	if s.resolveLexicalQNamePartsOwner != s {
		s.resolveLexicalQNamePartsFunc = s.resolveLexicalQNameParts
		s.resolveLexicalQNamePartsOwner = s
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
	if err := s.checkDocumentDepth(); err != nil {
		return err
	}
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
	xmlDocument := s.doc.xmlDocumentState
	xmlDocument.Reset(maxRetainedSliceCap)
	schemaLocationHints := s.doc.schemaLocationHints
	schemaLocationHints.Reset(maxRetainedMapLen)
	identity := s.doc.identity
	identity.Reset(maxRetainedMapLen, maxRetainedSliceCap)
	s.doc = documentState{
		xmlDocumentState:    xmlDocument,
		errors:              resetRetainedSlice(s.doc.errors),
		stack:               resetRetainedSlice(s.doc.stack),
		text:                resetRetainedBytes(s.doc.text),
		namePath:            resetRetainedSlice(s.doc.namePath),
		allBits:             resetRetainedSlice(s.doc.allBits),
		identity:            identity,
		schemaLocationHints: schemaLocationHints,
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
	if err := s.checkDocumentDepth(); err != nil {
		return err
	}
	se, err := s.doc.PrepareStart(se, &s.valueStrings, xmlDocumentLimits{
		depth:      s.maxInstanceDepth,
		attributes: s.maxInstanceAttributes,
	}, line, col)
	if err != nil {
		return err
	}
	xsiFlags := xsiStartAttributeFlagsFor(se.Attr)
	if xsiFlags.SchemaLocation {
		if schemaLocationErr := s.recordSchemaLocationHints(se.Attr, line, col); schemaLocationErr != nil {
			recoverErr := s.recover(schemaLocationErr)
			if recoverErr != nil {
				return recoverErr
			}
		}
	}
	rn := s.runtimeName(se.Name)
	hasXSIType := xsiFlags.Type
	elem, typ, skip, err := s.startType(rn, se, hasXSIType, line, col)
	if err != nil {
		return err
	}
	var nilled, elementValueKnown, elementDeclared, elementHasValueConstraint bool
	if s.schema == nil {
		input := ElementInput{
			ResolveQNameParts: s.qnameResolverForAttrs(hasXSIType),
			HasSchemaLocation: s.schemaLocationHintLookup(),
			Values:            &s.valueStrings,
			Context:           s.startContext(line, col),
			Element:           elem,
			Type:              typ,
			Skip:              skip,
		}
		start, startErr := ElementStart(s.rt, se.Attr, input)
		if startErr != nil {
			recoverErr := s.recover(startErr)
			if recoverErr != nil {
				return recoverErr
			}
		}
		elem = start.Element
		typ = start.Type
		skip = start.Skip
		s.pushFrame(elem, typ, start.Nilled, skip, start.ElementValueKnown, start.ElementDeclared, start.ElementHasValueConstraint)
		goto finish
	}

	if !skip {
		ctx := s.startContext(line, col)
		var decl runtime.ElementStartInfo
		declared := runtime.ValidElementID(elem, len(s.schema.ElementStartInfos))
		if declared {
			decl = s.schema.ElementStartInfos[elem]
		}
		elementValueKnown = true
		elementDeclared = declared
		elementHasValueConstraint = declared && (decl.Fixed || decl.Default)
		if declared && decl.Abstract {
			err = validation(ctx, xsderrors.CodeValidationElement, "abstract element cannot appear directly")
			recoverErr := s.recover(err)
			if recoverErr != nil {
				return recoverErr
			}
			elem = runtime.NoElement
			typ = s.schema.AnyType()
			skip = true
			elementValueKnown = false
			elementDeclared = false
			elementHasValueConstraint = false
		} else {
			typ, nilled, err = s.schemaElementStart(se.Attr, elem, typ, xsiFlags, decl, declared, ctx)
			if err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					return recoverErr
				}
			}
		}
	}
	s.pushSchemaFrame(elem, typ, nilled, skip, elementValueKnown, elementDeclared, elementHasValueConstraint)

finish:
	if len(s.doc.stack) != s.doc.Depth()+1 {
		return xsderrors.InternalInvariant("XML document depth does not match schema frame depth")
	}
	pathName := rn.Local
	if !skip && !rn.Known && rn.NS != "" {
		pathName = rn.Label()
	}
	s.doc.CommitStart(se.Name, se.RawName, pathName)
	if s.hasIdentityConstraints {
		s.doc.namePath = append(s.doc.namePath, rn)
		if scopeErr := s.startIdentityScope(elem, line, col); scopeErr != nil {
			return scopeErr
		}
		if matchErr := s.matchIdentitySelectors(line, col); matchErr != nil {
			return matchErr
		}
	}
	if !skip {
		if attrErr := s.validateAttributes(typ, se.Attr, line, col); attrErr != nil {
			return attrErr
		}
	}
	return nil
}

func (s *session) startType(rn runtime.RuntimeName, se stream.StartElement, hasXSIType bool, line, col int) (runtime.ElementID, runtime.TypeID, bool, error) {
	if s.doc.Depth() == 0 {
		return s.rootStartType(rn, se, line, col)
	}
	parent := &s.doc.stack[len(s.doc.stack)-1]
	parent.HasChild = true
	accepted, err := s.acceptChild(parent, rn, hasXSIType, line, col)
	if err == nil {
		return accepted.element, accepted.typ, accepted.skip, nil
	}
	if !accepted.recover {
		return runtime.NoElement, runtime.TypeID{}, false, err
	}
	recoverErr := s.recover(err)
	if recoverErr != nil {
		return runtime.NoElement, runtime.TypeID{}, false, recoverErr
	}
	return accepted.element, accepted.typ, accepted.skip, nil
}

func (s *session) rootStartType(rn runtime.RuntimeName, se stream.StartElement, line, col int) (runtime.ElementID, runtime.TypeID, bool, error) {
	hasXSIType := HasXSITypeAttribute(se.Attr)
	input := RootInput{
		Name:              se.Name,
		RuntimeName:       rn,
		Values:            &s.valueStrings,
		ResolveQNameParts: s.qnameResolverForAttrs(hasXSIType),
		HasSchemaLocation: s.schemaLocationHintLookup(),
		Context:           s.startContext(line, col),
	}
	var start StartResult
	var err error
	if s.schema != nil {
		start, err = RootStart(s.schema, se.Attr, input)
	} else {
		start, err = RootStart(s.rt, se.Attr, input)
	}
	if err != nil {
		if !start.Recover {
			return runtime.NoElement, runtime.TypeID{}, false, err
		}
		if recoverErr := s.recover(err); recoverErr != nil {
			return runtime.NoElement, runtime.TypeID{}, false, recoverErr
		}
	}
	return start.Element, start.Type, start.Skip, nil
}

func (s *session) schemaElementStart(
	attrs []stream.Attr,
	elem runtime.ElementID,
	typ runtime.TypeID,
	xsiFlags xsiStartAttributeFlags,
	decl runtime.ElementStartInfo,
	declared bool,
	ctx StartContext,
) (runtime.TypeID, bool, error) {
	nilled := false
	nilSpecified := false
	if xsiFlags.hasStartAttribute() {
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
					return typ, false, validation(ctx, xsderrors.CodeValidationNil, "invalid xsi:nil value")
				}
				nilled = parsed
			case vocab.XSIAttrType:
				override, err := resolveXSIType(s.schema, value, s.qnameResolverForAttrs(xsiFlags.Type), s.schemaLocationHintLookup(), ctx)
				if err != nil {
					return typ, nilled, err
				}
				if err := validateXSITypeOverride(s.schema, elem, typ, override, ctx); err != nil {
					return typ, nilled, err
				}
				typ = override
			}
		}
	}
	if typ.Kind == runtime.TypeComplex {
		id := runtime.ComplexTypeID(typ.ID)
		if !runtime.ValidComplexTypeID(id, len(s.schema.ComplexTypeInfos)) {
			return typ, nilled, xsderrors.InternalInvariant("start type metadata is invalid")
		}
		info := s.schema.ComplexTypeInfos[id]
		if info.Abstract {
			return typ, nilled, validation(ctx, xsderrors.CodeValidationType, "complex type is abstract")
		}
	}
	if nilSpecified && declared && !decl.Nillable {
		return typ, nilled, validation(ctx, xsderrors.CodeValidationNil, "element is not nillable")
	}
	if nilled {
		if !declared {
			return typ, nilled, validation(ctx, xsderrors.CodeValidationNil, "element is not nillable")
		}
		if decl.Fixed {
			return typ, nilled, validation(ctx, xsderrors.CodeValidationNil, "nilled element cannot have fixed value")
		}
	}
	return typ, nilled, nil
}

func (s *session) startContext(line, col int) StartContext {
	return s.doc.context(line, col)
}

func (s *session) checkDocumentDepth() error {
	if s.doc.Depth() != len(s.doc.stack) {
		return xsderrors.InternalInvariant("XML document depth does not match schema frame depth")
	}
	return nil
}

func (s *session) pushFrame(
	elem runtime.ElementID,
	typ runtime.TypeID,
	nilled, skip bool,
	elementValueKnown, elementDeclared, elementHasValueConstraint bool,
) {
	if s.schema != nil {
		s.pushSchemaFrame(elem, typ, nilled, skip, elementValueKnown, elementDeclared, elementHasValueConstraint)
		return
	}
	contentFrame := runtime.ContentFrameForType(s.rt, typ)
	childContent, childContentOK := s.childContentInfo(typ)
	if !childContentOK {
		childContent = runtime.ChildContentInfo{}
	}
	simpleContent, hasSimpleContent, simpleContentKnown := s.frameSimpleContent(typ, childContent, childContentOK)
	bitLen := contentFrame.AllBitLen()
	bitBase := len(s.doc.allBits)
	if bitLen > 0 {
		s.doc.allBits = slices.Grow(s.doc.allBits, bitLen)
		s.doc.allBits = s.doc.allBits[:bitBase+bitLen]
		clear(s.doc.allBits[bitBase:])
	}
	s.doc.stack = append(s.doc.stack, frame{
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
	})
}

func (s *session) pushSchemaFrame(
	elem runtime.ElementID,
	typ runtime.TypeID,
	nilled, skip bool,
	elementValueKnown, elementDeclared, elementHasValueConstraint bool,
) {
	contentFrame := s.schema.ContentFrameForPublishedSchema(typ)
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
	s.doc.stack = append(s.doc.stack, frame{
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
	})
}

func (s *session) schemaChildContentInfo(typ runtime.TypeID) (runtime.ChildContentInfo, bool) {
	content, ok := runtime.ElementChildContentByType(len(s.schema.SimpleTypePrimitives), s.schema.ComplexChildContentReads, typ)
	if !ok {
		return runtime.ChildContentInfo{}, false
	}
	return runtime.NewChildContentInfoForElementChildContent(content), true
}

func (s *session) frameSimpleContent(
	typ runtime.TypeID,
	child runtime.ChildContentInfo,
	childOK bool,
) (runtime.SimpleTypeID, bool, bool) {
	if s.schema == nil {
		return runtime.NoSimpleType, false, false
	}
	if id, ok := typ.Simple(); ok {
		return id, true, true
	}
	if !childOK {
		return runtime.NoSimpleType, false, false
	}
	if !child.Simple {
		return runtime.NoSimpleType, false, true
	}
	id, hasSimpleContent, ok := runtime.SimpleContentTypeByType(
		len(s.schema.SimpleTypePrimitives),
		s.schema.ComplexSimpleContentReads,
		typ,
	)
	if !ok {
		return runtime.NoSimpleType, false, false
	}
	return id, hasSimpleContent, true
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
	id, hasSimpleContent, ok := s.schema.SimpleContentType(typ)
	if !ok {
		return runtime.NoSimpleType, false, false
	}
	return id, hasSimpleContent, true
}

func (s *session) childContentInfo(typ runtime.TypeID) (runtime.ChildContentInfo, bool) {
	if s.schema != nil {
		return s.schema.ChildContent(typ)
	}
	return s.rt.ChildContent(typ)
}

func (s *session) chars(line, col int, data []byte, cdata bool) error {
	if err := s.checkDocumentDepth(); err != nil {
		return err
	}
	if len(s.doc.stack) == 0 {
		_, err := ValidateCharacterData(CharacterDataInput{
			Data:    data,
			Context: s.startContext(line, col),
			CDATA:   cdata,
		})
		return err
	}
	f := &s.doc.stack[len(s.doc.stack)-1]
	if len(data) == 0 || f.Skip {
		return nil
	}
	if f.Nilled {
		return validation(s.startContext(line, col), xsderrors.CodeValidationNil, "nilled element must be empty")
	}
	if frameHasSimpleContent(f) {
		return s.appendText(data, line, col)
	}
	var content runtime.ElementTextContent
	var ok bool
	if s.schema != nil {
		content, ok = s.schema.ElementTextContent(f.Type, f.Element)
	} else {
		content, ok = s.rt.ElementTextContent(f.Type, f.Element)
	}
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
