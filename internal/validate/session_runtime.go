package validate

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"sync/atomic"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	maxRetainedSliceCap  = 4096
	maxRetainedBufferCap = 1 << 20
	maxRetainedMapLen    = 4096
)

var errSemanticStop = errors.New("semantic validation stopped after reaching MaxErrors")

// Session validates XML instance documents against one compiled runtime.
//
// Concurrent use of one Session is rejected. Use separate sessions for
// concurrent validation.
type Session struct {
	session session
	inUse   atomic.Bool
}

// NewSession creates a reusable validation session.
func NewSession(rt *runtime.Schema, opts Options) (*Session, error) {
	result := new(Session)
	if err := initializeSession(&result.session, rt, opts); err != nil {
		return nil, err
	}
	return result, nil
}

func initializeSession(s *session, rt *runtime.Schema, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	if rt == nil {
		return xsderrors.InternalInvariant("nil validation schema")
	}
	*s = session{
		rt:     rt,
		limits: limits,
	}
	s.doc.identity = newIdentityEvaluation(rt, identityLimits{
		Entries:    limits.IdentityEntries,
		TupleBytes: limits.IdentityTupleBytes,
	}, limits.IdentityScopes)
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

// Validate validates one XML instance document. It clears document-local state
// before returning and may retain bounded scratch buffers and string caches for
// reuse.
func (s *Session) Validate(r io.Reader) error {
	if s == nil {
		return (*session)(nil).validate(r)
	}
	if !s.inUse.CompareAndSwap(false, true) {
		return xsderrors.Validation(xsderrors.CodeValidationSession, "validation session is already in use", nil)
	}
	// Defers run in LIFO order: cleanup must finish before copies can enter.
	defer s.inUse.Store(false)
	defer s.session.reset()
	return s.session.validate(r)
}

// session holds the state for validating documents against one Engine.
// Per-document state lives in doc; everything else is retained across
// documents: options, the reader buffer and parser, and the string caches.
type session struct {
	rt                           *runtime.Schema
	resolveLexicalQNamePartsFunc runtime.ResolveQNameParts
	doc                          documentState
	nameStrings                  stream.Cache
	valueStrings                 stream.Cache
	derivationScratch            runtime.TypeDerivationScratch
	stringPatternScratch         runtime.StringPatternScratch
	attributeSeen                []bool
	parser                       stream.Parser
	limits                       Limits
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
	identity            identityEvaluation
	schemaLocationHints SchemaLocationHints
	allBits             []uint64
	errors              []error
	text                []byte
	syntaxOnly          bool
}

type frame struct {
	Index             int
	BitBase           int
	BitLen            int
	TextStart         int
	Content           runtime.ContentState
	Type              runtime.TypeID
	SimpleContent     runtime.SimpleTypeID
	Element           runtime.ElementID
	TextContent       runtime.ElementTextContent
	Nilled            bool
	Mode              elementMode
	HasChild          bool
	HasText           bool
	AssessmentInvalid bool
}

type elementMode uint8

const (
	elementRecovery elementMode = iota
	elementAssessed
	elementWildcardSkipped
)

func (s *session) validate(r io.Reader) error {
	if s == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	defer s.parser.Detach()
	if s.rt == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	if err := s.resetParser(r); err != nil {
		return err
	}
	return s.validateTokens()
}

func (s *session) resetParser(r io.Reader) error {
	if err := s.parser.ResetWithConfig(r, &s.nameStrings, &s.valueStrings, stream.Config{
		Limits: stream.Limits{
			MaxInputBytes: s.limits.InstanceBytes,
			MaxTokenBytes: s.limits.InstanceTokenBytes,
			MaxAttrs:      s.limits.InstanceAttributes,
		},
		LazyAttrValues: true,
	}); err != nil {
		return instanceReaderError(err)
	}
	return nil
}

func (s *session) validateTokens() error {
	for {
		tok, err := s.parser.Next()
		if err != nil {
			return s.finishTokenStream(tok, err)
		}
		syntaxOnly := s.doc.syntaxOnly
		if err := s.validateToken(tok, syntaxOnly); err != nil {
			return err
		}
		if !syntaxOnly && s.doc.syntaxOnly {
			s.discardSemanticState()
		}
	}
}

func (s *session) finishTokenStream(tok stream.Token, err error) error {
	if stream.IsOnlyEOF(err) {
		return s.finishValidation()
	}
	return s.parseError(tok, err)
}

func (s *session) validateToken(tok stream.Token, syntaxOnly bool) error {
	switch tok.Kind {
	case stream.KindStart:
		return s.start(tok.Line, tok.Column, tok.Start)
	case stream.KindEnd:
		return s.end(tok.Line, tok.Column, tok.End)
	case stream.KindCharData:
		return s.validateCharacterToken(tok, syntaxOnly)
	case stream.KindDirective:
		return ValidateDirective(s.startContext(tok.Line, tok.Column), tok.Directive)
	default:
		return nil
	}
}

func (s *session) validateCharacterToken(tok stream.Token, syntaxOnly bool) error {
	err := s.chars(tok.Line, tok.Column, tok.Data, tok.CDATA)
	if err == nil || syntaxOnly {
		return err
	}
	err = s.recoverAssessment(err)
	if errors.Is(err, errSemanticStop) {
		return nil
	}
	return err
}

func (s *session) parseError(tok stream.Token, err error) error {
	line, col := tok.Line, tok.Column
	if line == 0 {
		line, col = s.parser.Pos()
	}
	return StreamError(line, col, s.doc.PathString(), err)
}

func (s *session) finishValidation() error {
	if err := s.doc.Complete(); err != nil {
		return err
	}
	if !s.doc.syntaxOnly {
		if err := s.checkIDRefs(); err != nil {
			if errors.Is(err, errSemanticStop) {
				s.discardSemanticState()
				return s.result()
			}
			return err
		}
	}
	return s.result()
}

// reset rebuilds the per-document state. Fields named in the literal recycle
// bounded capacity from the previous document; every other documentState
// field is zeroed by the literal itself, so omitting a field can never leak
// state across documents.
func (s *session) reset() {
	s.parser.Detach()
	s.derivationScratch.Reset(maxRetainedMapLen)
	s.stringPatternScratch.Reset(maxRetainedSliceCap)
	xmlDocument := s.doc.xmlDocument
	xmlDocument.Reset(maxRetainedSliceCap)
	schemaLocationHints := s.doc.schemaLocationHints
	schemaLocationHints.Reset(maxRetainedMapLen)
	identity := s.doc.identity
	identity.reset(maxRetainedMapLen, maxRetainedSliceCap)
	s.doc = documentState{
		xmlDocument:         xmlDocument,
		errors:              resetRetainedReferences(s.doc.errors, maxRetainedSliceCap),
		text:                resetRetainedBytes(s.doc.text),
		allBits:             resetRetainedValues(s.doc.allBits, maxRetainedSliceCap),
		identity:            identity,
		schemaLocationHints: schemaLocationHints,
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
		return xsderrors.NewErrors(s.doc.errors...)
	}
}

func (s *session) recover(err error) error {
	if err == nil {
		return nil
	}
	if !RecoverableError(err) {
		return err
	}
	if !RecoveryLimitReached(len(s.doc.errors), s.limits.Errors) {
		s.doc.errors = append(s.doc.errors, err)
		if RecoveryLimitReached(len(s.doc.errors), s.limits.Errors) {
			s.doc.syntaxOnly = true
			return errSemanticStop
		}
	}
	return nil
}

func (s *session) recoverAssessment(err error) error {
	if assessmentFailure(err) {
		if current, ok := s.doc.Current(); ok && current.Mode == elementAssessed {
			current.AssessmentInvalid = true
		}
	}
	return s.recover(err)
}

func assessmentFailure(err error) bool {
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	return ok && diagnostic != nil &&
		diagnostic.Category() == xsderrors.CategoryValidation &&
		diagnostic.Code() != xsderrors.CodeValidationIdentity &&
		diagnostic.Code() != xsderrors.CodeValidationLimit
}

func (s *session) discardSemanticState() {
	s.doc.clearPayloads()
	s.doc.identity.discard()
	s.doc.schemaLocationHints = SchemaLocationHints{}
	s.doc.allBits = nil
	s.doc.text = nil
	s.attributeSeen = nil
}

type startTransactionPhase uint8

const (
	startTransactionPrepared startTransactionPhase = iota
	startTransactionXMLCommitted
	startTransactionDone
)

//nolint:govet // Fields are grouped by the state restored together.
type sessionStartTransaction struct {
	s                 *session
	xml               xmlDocumentCheckpoint
	hints             SchemaLocationHints
	transition        runtime.ContentTransition
	namespace         xmlns.Frame
	allBitsLen        int
	errorsLen         int
	parentIndex       int
	syntaxOnly        bool
	invalidatesParent bool
	phase             startTransactionPhase
}

func (s *session) beginStartTransaction(xmlCheckpoint xmlDocumentCheckpoint, namespace xmlns.Frame) (sessionStartTransaction, error) {
	transaction := sessionStartTransaction{
		s:           s,
		xml:         xmlCheckpoint,
		hints:       s.doc.schemaLocationHints,
		allBitsLen:  len(s.doc.allBits),
		errorsLen:   len(s.doc.errors),
		parentIndex: xmlCheckpoint.depth - 1,
		namespace:   namespace,
		syntaxOnly:  s.doc.syntaxOnly,
	}
	if err := s.doc.identity.beginStart(); err != nil {
		return sessionStartTransaction{}, err
	}
	return transaction, nil
}

func (t *sessionStartTransaction) stageContent(accepted acceptedChild) {
	t.transition = accepted.transition
	t.invalidatesParent = accepted.invalidatesParent
}

func (t *sessionStartTransaction) commitXMLStart(start preparedXMLStart, expandedPath bool, payload frame) error {
	if t.phase != startTransactionPrepared || t.s.doc.Depth() != t.xml.depth {
		return xsderrors.InternalInvariant("XML start transaction phase is invalid")
	}
	t.s.doc.CommitStart(start, expandedPath, payload)
	t.phase = startTransactionXMLCommitted
	return nil
}

func (t *sessionStartTransaction) commit() error {
	if t.phase != startTransactionXMLCommitted {
		return xsderrors.InternalInvariant("start transaction commit phase is invalid")
	}
	parent, _, err := t.parentFrame()
	if err != nil {
		return err
	}
	if err := t.validateContentTransition(parent); err != nil {
		return err
	}
	if err := t.s.doc.identity.validateStartCommit(); err != nil {
		return err
	}
	if err := t.commitContentTransition(parent); err != nil {
		return err
	}
	t.s.doc.identity.commitStart()
	t.commitParentState(parent)
	t.phase = startTransactionDone
	return nil
}

func (t *sessionStartTransaction) parentFrame() (*frame, bool, error) {
	if t.parentIndex < 0 {
		return nil, false, nil
	}
	if t.parentIndex >= len(t.s.doc.elements) {
		return nil, false, xsderrors.InternalInvariant("start transaction parent frame is invalid")
	}
	return &t.s.doc.elements[t.parentIndex].payload, true, nil
}

func (t *sessionStartTransaction) validateContentTransition(parent *frame) error {
	if !t.transition.IsPlanned() {
		return nil
	}
	if parent == nil {
		return xsderrors.InternalInvariant("root start has a parent content transition")
	}
	scratch := t.s.contentScratch(parent)
	if !t.transition.CanCommit(parent.Content, &scratch) {
		return xsderrors.InternalInvariant("parent content transition is stale")
	}
	return nil
}

func (t *sessionStartTransaction) commitContentTransition(parent *frame) error {
	if !t.transition.IsPlanned() {
		return nil
	}
	scratch := t.s.contentScratch(parent)
	if !t.transition.Commit(&parent.Content, &scratch) {
		return xsderrors.InternalInvariant("parent content transition commit failed")
	}
	return nil
}

func (t *sessionStartTransaction) commitParentState(parent *frame) {
	if parent != nil {
		if t.invalidatesParent {
			parent.AssessmentInvalid = true
		}
		parent.HasChild = true
	}
}

func (t *sessionStartTransaction) stopSemanticValidation(start preparedXMLStart) error {
	t.restoreSemanticState(false)
	switch t.phase {
	case startTransactionPrepared:
		if err := t.commitXMLStart(start, false, frame{}); err != nil {
			return err
		}
	case startTransactionXMLCommitted:
		t.s.doc.clearCurrentPayload()
	default:
		return xsderrors.InternalInvariant("start transaction stop phase is invalid")
	}
	t.phase = startTransactionDone
	return nil
}

func (t *sessionStartTransaction) abort() error {
	if t.phase == startTransactionDone {
		return nil
	}
	t.restoreSemanticState(true)
	err := t.s.doc.rollbackStart(t.xml, t.namespace)
	t.phase = startTransactionDone
	return err
}

func (t *sessionStartTransaction) restoreSemanticState(restoreRecovery bool) {
	t.s.doc.identity.abortStart()
	t.s.doc.schemaLocationHints = t.hints
	clear(t.s.doc.allBits[t.allBitsLen:])
	t.s.doc.allBits = t.s.doc.allBits[:t.allBitsLen]
	if restoreRecovery {
		clear(t.s.doc.errors[t.errorsLen:])
		t.s.doc.errors = t.s.doc.errors[:t.errorsLen]
		t.s.doc.syntaxOnly = t.syntaxOnly
	}
}

func (s *session) start(line, col int, token stream.StartElement) error {
	if s.doc.syntaxOnly {
		return s.syntaxStart(line, col, token)
	}
	xmlCheckpoint := s.doc.startCheckpoint()
	se, err := s.doc.PrepareStart(token, &s.valueStrings, s.limits.InstanceDepth, line, col)
	if err != nil {
		return err
	}
	transaction, err := s.beginStartTransaction(xmlCheckpoint, se.namespace)
	if err != nil {
		if abortErr := s.doc.AbortStart(se); abortErr != nil {
			return errors.Join(err, abortErr)
		}
		return err
	}
	resultErr := s.runStartTransaction(&transaction, se, token, line, col)
	if abortErr := transaction.abort(); abortErr != nil {
		return errors.Join(resultErr, abortErr)
	}
	return resultErr
}

func (s *session) runStartTransaction(
	transaction *sessionStartTransaction,
	se preparedXMLStart,
	token stream.StartElement,
	line, col int,
) error {
	xsiFlags := xsiStartAttributeFlagsFor(token.Attr)
	if xsiFlags.SchemaLocation {
		if err := s.recover(s.recordSchemaLocationHints(token.Attr, line, col)); err != nil {
			return s.handleStartTransactionError(transaction, se, err)
		}
	}
	rn := s.runtimeName(se.name)
	accepted, err := s.startType(rn, se, token, xsiFlags.Type, line, col)
	if err != nil {
		return s.handleStartTransactionError(transaction, se, err)
	}
	transaction.stageContent(accepted)
	start := accepted.start
	nilled, err := s.assessElementStart(&start, token.Attr, xsiFlags, s.startContext(line, col))
	if err != nil {
		return s.handleStartTransactionError(transaction, se, err)
	}
	schemaFrame, err := s.newSchemaFrame(start, nilled)
	if err != nil {
		return err
	}
	if err := transaction.commitXMLStart(se, expandedInstancePath(start, rn), schemaFrame); err != nil {
		return err
	}
	if identityErr := s.startFrameIdentity(start, rn, schemaFrame, line, col); identityErr != nil {
		return s.handleStartTransactionError(transaction, se, identityErr)
	}
	if attrErr := s.validateStartAttributes(start, token.Attr, line, col); attrErr != nil {
		return s.handleStartTransactionError(transaction, se, attrErr)
	}
	return transaction.commit()
}

func (s *session) handleStartTransactionError(transaction *sessionStartTransaction, start preparedXMLStart, err error) error {
	if errors.Is(err, errSemanticStop) {
		return transaction.stopSemanticValidation(start)
	}
	return err
}

func (s *session) assessElementStart(
	start *schemaStart,
	attrs []stream.Attr,
	flags xsiStartAttributeFlags,
	ctx StartContext,
) (bool, error) {
	if start.mode != elementAssessed {
		return false, nil
	}
	decl, declared := s.rt.Element(start.element)
	info, complete, err := s.initialElementAssessment(start, decl, declared, ctx)
	if complete {
		return false, err
	}
	var nilValue, typeValue string
	for i := range attrs {
		a := &attrs[i]
		switch xsiStartValueFor(a.Name) {
		case xsiStartNilValue:
			nilValue = a.StringValue(&s.valueStrings)
		case xsiStartTypeValue:
			typeValue = a.StringValue(&s.valueStrings)
		case xsiStartNoValue:
		}
	}
	nilled, err := s.assessXSINil(start, flags.Nil, nilValue, ctx)
	if err != nil {
		return false, err
	}
	info, complete, err = s.assessXSIType(start, decl, declared, flags.Type, typeValue, info, ctx)
	if complete {
		return nilled, err
	}
	var effectiveErr error
	start.typ, nilled, effectiveErr = validateElementEffectiveState(
		decl, declared, start.typ, nilled, flags.Nil, info, true, ctx,
	)
	return nilled, s.recoverElementStartAssessment(start, effectiveErr)
}

func expandedInstancePath(start schemaStart, rn runtime.RuntimeName) bool {
	return start.mode == elementAssessed && !rn.Known && rn.NS != ""
}

func (s *session) startFrameIdentity(start schemaStart, rn runtime.RuntimeName, f frame, line, col int) error {
	return s.doc.identity.startElement(identityElementStart{
		Name:          rn,
		Element:       f.Element,
		Mode:          start.mode,
		Context:       s.startContext(line, col),
		Nilled:        f.Nilled,
		SimpleContent: f.SimpleContent != runtime.NoSimpleType,
	})
}

func (s *session) validateStartAttributes(start schemaStart, attrs []stream.Attr, line, col int) error {
	switch start.mode {
	case elementAssessed:
		return s.validateAttributes(start.typ, attrs, line, col)
	case elementWildcardSkipped:
		return s.rejectUnassessedIdentityAttributes(attrs, line, col, true)
	case elementRecovery:
		return s.rejectUnassessedIdentityAttributes(attrs, line, col, false)
	default:
		return xsderrors.InternalInvariant("element assessment mode is invalid")
	}
}

func (s *session) syntaxStart(line, col int, token stream.StartElement) error {
	start, err := s.doc.PrepareStart(token, &s.valueStrings, s.limits.InstanceDepth, line, col)
	if err != nil {
		return err
	}
	s.doc.CommitStart(start, false, frame{})
	return nil
}

func (s *session) initialElementAssessment(start *schemaStart, decl runtime.ElementStartInfo, declared bool, ctx StartContext) (runtime.TypeInfo, bool, error) {
	if declared && decl.Abstract {
		*start = recoverySchemaStart()
		err := validation(ctx, xsderrors.CodeValidationElement, "abstract element cannot appear directly")
		return runtime.TypeInfo{}, true, s.recoverElementStartAssessment(start, err)
	}
	info, known := s.rt.TypeInfo(start.typ)
	if !known {
		return runtime.TypeInfo{}, true, xsderrors.InternalInvariant("start type metadata is invalid")
	}
	if info.Unavailable {
		*start = recoverySchemaStart()
		err := validation(ctx, xsderrors.CodeValidationElement, "element type is unavailable")
		return runtime.TypeInfo{}, true, s.recoverElementStartAssessment(start, err)
	}
	return info, false, nil
}

type xsiStartValue uint8

const (
	xsiStartNoValue xsiStartValue = iota
	xsiStartNilValue
	xsiStartTypeValue
)

func xsiStartValueFor(name xml.Name) xsiStartValue {
	if name.Space != vocab.XSINamespaceURI {
		return xsiStartNoValue
	}
	switch name.Local {
	case vocab.XSIAttrNil:
		return xsiStartNilValue
	case vocab.XSIAttrType:
		return xsiStartTypeValue
	default:
		return xsiStartNoValue
	}
}

func (s *session) assessXSINil(start *schemaStart, specified bool, value string, ctx StartContext) (bool, error) {
	if !specified {
		return false, nil
	}
	nilled, ok := ParseXSINil(value)
	if ok {
		return nilled, nil
	}
	err := validation(ctx, xsderrors.CodeValidationNil, "invalid xsi:nil value")
	return false, s.recoverElementStartAssessment(start, err)
}

func (s *session) assessXSIType(start *schemaStart, decl runtime.ElementStartInfo, declared, specified bool, value string, info runtime.TypeInfo, ctx StartContext) (runtime.TypeInfo, bool, error) {
	if !specified {
		return info, false, nil
	}
	override, err := resolveXSIType(s.rt, value, s.qnameResolver(), s.schemaLocationHintLookup(), ctx)
	if err != nil {
		return s.recoverXSITypeError(start, info, err)
	}
	overrideInfo, known := s.rt.TypeInfo(override)
	if !known {
		return info, true, xsderrors.InternalInvariant("start type metadata is invalid")
	}
	if overrideInfo.Unavailable {
		*start = recoverySchemaStart()
		err := validation(ctx, xsderrors.CodeValidationElement, "element type is unavailable")
		return info, true, s.recoverElementStartAssessment(start, err)
	}
	if err := validateXSITypeOverride(s.rt, start.typ, override, decl.Block, declared, &s.derivationScratch, ctx); err != nil {
		return s.recoverXSITypeError(start, info, err)
	}
	start.typ = override
	return overrideInfo, false, nil
}

func (s *session) recoverXSITypeError(start *schemaStart, info runtime.TypeInfo, err error) (runtime.TypeInfo, bool, error) {
	err = s.recoverElementStartAssessment(start, err)
	return info, err != nil, err
}

func (s *session) recoverElementStartAssessment(start *schemaStart, err error) error {
	if assessmentFailure(err) {
		start.invalid = true
	}
	return s.recover(err)
}

type schemaStart struct {
	element runtime.ElementID
	typ     runtime.TypeID
	mode    elementMode
	invalid bool
}

func assessedSchemaStart(element runtime.ElementID, typ runtime.TypeID) schemaStart {
	return schemaStart{element: element, typ: typ, mode: elementAssessed}
}

func wildcardSkippedSchemaStart() schemaStart {
	return schemaStart{element: runtime.NoElement, mode: elementWildcardSkipped}
}

func recoverySchemaStart() schemaStart {
	return schemaStart{element: runtime.NoElement, mode: elementRecovery}
}

func (s *session) startType(rn runtime.RuntimeName, se preparedXMLStart, token stream.StartElement, hasXSIType bool, line, col int) (acceptedChild, error) {
	if s.doc.Depth() == 0 {
		start, err := s.rootStartType(rn, se, token, hasXSIType, line, col)
		return acceptedChild{start: start}, err
	}
	parent, ok := s.doc.Current()
	if !ok {
		return acceptedChild{}, xsderrors.InternalInvariant("child start has no parent frame")
	}
	accepted, err := s.acceptChild(parent, rn, hasXSIType, line, col)
	if err == nil {
		return accepted, nil
	}
	accepted.invalidatesParent = assessmentFailure(err)
	recoverErr := s.recover(err)
	if recoverErr != nil {
		return acceptedChild{}, recoverErr
	}
	return accepted, nil
}

func (s *session) rootStartType(rn runtime.RuntimeName, se preparedXMLStart, token stream.StartElement, hasXSIType bool, line, col int) (schemaStart, error) {
	input := RootInput{
		Name:              se.name,
		RuntimeName:       rn,
		Values:            &s.valueStrings,
		ResolveQNameParts: s.qnameResolverForAttrs(hasXSIType),
		HasSchemaLocation: s.schemaLocationHintLookup(),
		Context:           s.startContext(line, col),
	}
	start, err := RootStart(s.rt, token.Attr, input)
	invalid := false
	if err != nil {
		if !start.Recover {
			return schemaStart{}, err
		}
		if recoverErr := s.recover(err); recoverErr != nil {
			return schemaStart{}, recoverErr
		}
		invalid = true
	}
	if start.Skip {
		return recoverySchemaStart(), nil
	}
	out := assessedSchemaStart(start.Element, start.Type)
	out.invalid = invalid
	return out, nil
}

func (s *session) startContext(line, col int) StartContext {
	return s.doc.context(line, col)
}

func (s *session) newSchemaFrame(
	start schemaStart,
	nilled bool,
) (frame, error) {
	if start.mode != elementAssessed {
		return frame{
			Element:       runtime.NoElement,
			SimpleContent: runtime.NoSimpleType,
			BitBase:       len(s.doc.allBits),
			TextStart:     len(s.doc.text),
			Mode:          start.mode,
		}, nil
	}
	elem := start.element
	typ := start.typ
	simpleContent, hasSimpleContent, ok := s.rt.SimpleContentType(typ)
	if !ok {
		return frame{}, xsderrors.InternalInvariant("simple content type metadata is invalid")
	}
	if !hasSimpleContent {
		simpleContent = runtime.NoSimpleType
	}
	textContent, ok := s.rt.ElementTextContent(typ, elem)
	if !ok {
		return frame{}, xsderrors.InternalInvariant("character data content info is invalid")
	}
	contentFrame := s.rt.ContentFrame(typ)
	bitLen := contentFrame.AllBitLen()
	bitBase := len(s.doc.allBits)
	if bitLen > 0 {
		s.doc.allBits = slices.Grow(s.doc.allBits, bitLen)
		s.doc.allBits = s.doc.allBits[:bitBase+bitLen]
		clear(s.doc.allBits[bitBase:])
	}
	return frame{
		Element:           elem,
		Type:              typ,
		BitBase:           bitBase,
		BitLen:            bitLen,
		Content:           contentFrame.ContentState(),
		TextContent:       textContent,
		SimpleContent:     simpleContent,
		TextStart:         len(s.doc.text),
		Nilled:            nilled,
		Mode:              elementAssessed,
		AssessmentInvalid: start.invalid,
	}, nil
}

func (s *session) chars(line, col int, data []byte, cdata bool) error {
	if s.doc.syntaxOnly && s.doc.Depth() != 0 {
		return nil
	}
	f, ok := s.doc.Current()
	if !ok {
		return ValidateDocumentCharacterData(data, cdata, s.startContext(line, col))
	}
	if len(data) == 0 || f.Mode != elementAssessed {
		return nil
	}
	if f.Nilled {
		return validation(s.startContext(line, col), xsderrors.CodeValidationNil, "nilled element must be empty")
	}
	return s.validateAssessedCharacterData(f, data, line, col)
}

func (s *session) validateAssessedCharacterData(f *frame, data []byte, line, col int) error {
	if f.SimpleContent != runtime.NoSimpleType {
		return s.appendText(data, line, col)
	}
	content := f.TextContent
	whitespace := lex.IsXMLWhitespaceBytes(data)
	if !whitespace {
		f.HasText = true
	}
	if content.AllowsMixedContent() {
		return s.captureMixedCharacterData(content, data, line, col)
	}
	if !whitespace {
		ctx := s.startContext(line, col)
		return validation(ctx, xsderrors.CodeValidationText, "character data is not allowed")
	}
	return nil
}

func (s *session) captureMixedCharacterData(content runtime.ElementTextContent, data []byte, line, col int) error {
	if content.HasFixedElementValue() {
		return s.appendText(data, line, col)
	}
	return nil
}

func (s *session) appendText(data []byte, line, col int) error {
	if s.limits.InstanceTextBytes > 0 && int64(len(s.doc.text)) > s.limits.InstanceTextBytes-int64(len(data)) {
		return validation(s.startContext(line, col), xsderrors.CodeValidationLimit, "instance text byte limit exceeded")
	}
	s.doc.text = append(s.doc.text, data...)
	return nil
}
