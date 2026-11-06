package validate

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"sync/atomic"

	"github.com/jacoelho/xsd/internal/lex"
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// Session high-water retention policy. Every reset, for explicit sessions and
// the pool's idle session alike, drops any slice, buffer, or map larger than
// these bounds instead of clearing it, so one large document cannot pin its
// peak memory across later validations.
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

// initializeSession populates a newly allocated zero session. Assigning only
// admitted configuration avoids clearing the embedded input buffer a second time.
func initializeSession(s *session, rt *xsdSchema.Schema, limits Limits) {
	s.rt = rt
	s.doc.identity = newIdentityEvaluation(rt, identityLimits{})
	s.setLimits(limits)
}

// Validate validates one XML instance document. It clears document-local state
// before returning and may retain bounded scratch buffers and string caches for
// reuse.
func (s *Session) Validate(r io.Reader) error {
	if s == nil {
		return (*session)(nil).validate(r)
	}
	if !s.inUse.CompareAndSwap(false, true) {
		return xsderrors.Validation(xsderrors.CodeValidationSession, msgSessionInUse, nil)
	}
	// Defers run in LIFO order: cleanup must finish before copies can enter.
	defer s.inUse.Store(false)
	defer s.session.reset()
	return s.session.validate(r)
}

// session holds the state for validating documents against one Engine.
// Per-document state lives in doc; everything else is retained across
// documents: options, the reader buffer and parser.
//
// Validation work buffers (derivation and value scratch) live here, never in the
// immutable schema, so one sealed schema serves concurrent sessions.
type session struct {
	rt                *xsdSchema.Schema
	valueResolver     xsdValue.Resolver
	doc               documentState
	derivationScratch xsdSchema.TypeDerivationScratch
	valueScratch      xsdValue.Scratch
	attributeSeen     []bool
	reader            xmlstream.Reader
	limits            Limits
	// patternTotal is the pattern-work account of the document in progress; it
	// is reset at every validate entry and passed by pointer at each call.
	patternTotal xsdValue.PatternAccount
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
	schemaLocationHints schemaLocationHintSet
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
	Content           xsdSchema.ContentState
	Type              xsdSchema.TypeID
	SimpleContent     xsdSchema.SimpleTypeID
	Element           xsdSchema.ElementID
	TextContent       xsdSchema.ElementTextContent
	Nilled            bool
	Mode              elementMode
	HasChild          bool
	HasText           bool
	AssessmentInvalid bool
}

type elementMode uint8

// The zero mode is skipped, so an unselected start is never assessed.
const (
	elementWildcardSkipped elementMode = iota
	elementAssessed
)

func (s *session) validate(r io.Reader) error {
	if s == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	// The reader owns borrowed token lifetimes; detach before any caller guard is released.
	defer s.reader.Detach()
	if s.rt == nil {
		return xsderrors.InternalInvariant("nil validation session")
	}
	// Every entry starts a fresh document account, including after a document
	// that failed mid-stream.
	s.patternTotal.Reset(s.limits.InstanceTotalPatternWork)
	if err := s.resetParser(r); err != nil {
		return err
	}
	return s.validateTokens()
}

func (s *session) resetParser(r io.Reader) error {
	return resetInstanceReader(&s.reader, r, s.limits)
}

func (s *session) validateTokens() error {
	for {
		tok, err := s.reader.Next()
		if err != nil {
			return s.finishTokenStream(err)
		}
		mode := tokenValidationSemantic
		if s.doc.syntaxOnly {
			mode = tokenValidationSyntaxOnly
		}
		if err := s.validateToken(tok, mode); err != nil {
			return err
		}
		if mode == tokenValidationSemantic && s.doc.syntaxOnly {
			s.discardSemanticState()
		}
	}
}

func (s *session) finishTokenStream(err error) error {
	if xmlstream.IsOnlyEOF(err) {
		return s.finishValidation()
	}
	return s.parseError(err)
}

type tokenValidationMode uint8

const (
	tokenValidationSemantic tokenValidationMode = iota
	tokenValidationSyntaxOnly
)

func (s *session) validateToken(tok *xmlstream.Token, mode tokenValidationMode) error {
	switch tok.Kind { //nolint:exhaustive // Reader.Next rejects directives before consumers see them.
	case xmlstream.KindStart:
		return s.start(tok.Line, tok.Column, tok.Start.Attr)
	case xmlstream.KindEnd:
		return s.end(tok.Line, tok.Column)
	case xmlstream.KindCharData:
		return s.validateCharacterToken(tok, mode)
	case xmlstream.KindComment, xmlstream.KindPI:
		return nil
	}
	return nil
}

func (s *session) validateCharacterToken(tok *xmlstream.Token, mode tokenValidationMode) error {
	err := s.chars(tok.Line, tok.Column, tok.Data)
	if err == nil || mode == tokenValidationSyntaxOnly {
		return err
	}
	err = s.recoverAssessment(err)
	if errors.Is(err, errSemanticStop) {
		return nil
	}
	return err
}

func (s *session) parseError(err error) error {
	line, col := streamErrorPosition(&s.reader, err)
	return streamError(err, s.startContext(line, col), s.limits)
}

func (s *session) finishValidation() error {
	if err := s.doc.Complete(&s.reader); err != nil {
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

// reset rebuilds document state after validate detaches the XML reader.
// Fields named in the literal recycle bounded capacity; every other documentState
// field is zeroed by the literal itself, so omitting a field can never leak
// state across documents.
func (s *session) reset() {
	s.derivationScratch.Reset(maxRetainedMapLen)
	s.valueScratch.Reset(maxRetainedSliceCap)
	// Identity values point into the document-owned retained-path store.
	// Clear them before resetting that store.
	identity := s.doc.identity
	identity.reset(maxRetainedMapLen, maxRetainedSliceCap)
	xmlDocument := s.doc.xmlDocument
	xmlDocument.Reset(maxRetainedSliceCap)
	schemaLocationHints := s.doc.schemaLocationHints
	schemaLocationHints.Reset(maxRetainedMapLen)
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

// recover records a recoverable diagnostic. Collected diagnostics form the
// invalid result, so recoverable means exactly conclusive as xsderrors
// assesses it; any other failure aborts the document. Reaching MaxErrors
// switches the document to syntax-only processing: semantic assessment stops,
// but XML admission continues to EOF so a later fatal XML or limit failure
// still takes precedence over the collected diagnostics.
func (s *session) recover(err error) error {
	if err == nil {
		return nil
	}
	if !xsderrors.IsConclusiveValidation(err) {
		return err
	}
	if !recoveryLimitReached(len(s.doc.errors), s.limits.Errors) {
		s.doc.errors = append(s.doc.errors, err)
		if recoveryLimitReached(len(s.doc.errors), s.limits.Errors) {
			s.doc.syntaxOnly = true
			return errSemanticStop
		}
	}
	return nil
}

func (s *session) recoverAssessment(err error) error {
	if err == nil {
		return nil
	}
	if assessmentFailure(err) {
		if current, ok := s.doc.Current(); ok && current.Mode == elementAssessed {
			current.AssessmentInvalid = true
		}
	}
	return s.recover(err)
}

// assessmentFailure reports whether err makes the current element's own
// assessment invalid. Identity failures belong to the constraint's scope
// owner, not to the element whose field or selector produced them.
func assessmentFailure(err error) bool {
	if !xsderrors.IsConclusiveValidation(err) {
		return false
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	return ok && diagnostic.Code() != xsderrors.CodeValidationIdentity
}

func (s *session) discardSemanticState() {
	s.doc.clearPayloads()
	s.doc.identity.discard()
	s.doc.discardRetainedPaths()
	s.doc.schemaLocationHints = schemaLocationHintSet{}
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

// sessionStartTransaction makes one element start atomic across the XML and
// namespace stacks, schema-location hints, the parent's content-model state and
// all-group bits, recovery state, and identity state. A fatal failure (abort)
// rolls every owner back to the checkpoint and aborts the reader's namespace
// frame. A semantic stop (MaxErrors reached) instead keeps only the committed
// XML syntax state needed to parse the remainder and discards semantic state.
//
//nolint:govet // Fields are grouped by the state restored together.
type sessionStartTransaction struct {
	s                 *session
	xml               xmlDocumentCheckpoint
	hints             schemaLocationHintSet
	transition        xsdSchema.ContentTransition
	handle            xmlstream.Handle
	allBitsLen        int
	errorsLen         int
	parentIndex       int
	syntaxOnly        bool
	invalidatesParent bool
	phase             startTransactionPhase
}

func (t *sessionStartTransaction) begin(s *session, xmlCheckpoint xmlDocumentCheckpoint, handle xmlstream.Handle) error {
	*t = sessionStartTransaction{
		s:           s,
		xml:         xmlCheckpoint,
		hints:       s.doc.schemaLocationHints,
		allBitsLen:  len(s.doc.allBits),
		errorsLen:   len(s.doc.errors),
		parentIndex: xmlCheckpoint.depth - 1,
		handle:      handle,
		syntaxOnly:  s.doc.syntaxOnly,
	}
	return s.doc.identity.beginStart()
}

func (t *sessionStartTransaction) stageContent(accepted *acceptedChild) {
	t.transition = accepted.transition
	t.invalidatesParent = accepted.invalidatesParent
}

func (t *sessionStartTransaction) commitXMLStart(start preparedXMLStart, pathMode xmlPathMode, payload frame) error {
	if t.phase != startTransactionPrepared || t.s.doc.Depth() != t.xml.depth {
		return xsderrors.InternalInvariant("XML start transaction phase is invalid")
	}
	switch pathMode {
	case xmlPathLexical:
		t.s.doc.CommitStart(start, payload)
	case xmlPathExpanded:
		t.s.doc.CommitExpandedStart(start, payload)
	case xmlPathInvalid:
		return xsderrors.InternalInvariant("XML path mode is invalid")
	default:
		err := xsderrors.InternalInvariant("XML path mode is invalid")
		return err
	}
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
	if t.transition.IsPlanned() && parent == nil {
		return xsderrors.InternalInvariant("root start has a parent content transition")
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
	t.restoreStartState()
	switch t.phase {
	case startTransactionPrepared:
		if err := t.commitXMLStart(start, xmlPathLexical, frame{}); err != nil {
			return err
		}
	case startTransactionXMLCommitted:
		t.s.doc.clearCurrentPayload()
	case startTransactionDone:
		return xsderrors.InternalInvariant("start transaction stop phase is invalid")
	default:
		err := xsderrors.InternalInvariant("start transaction stop phase is invalid")
		return err
	}
	t.phase = startTransactionDone
	return nil
}

func (t *sessionStartTransaction) abort() error {
	if t.phase == startTransactionDone {
		return nil
	}
	t.restoreStartState()
	t.restoreRecoveryState()
	t.s.doc.rollbackStart(t.xml)
	var err error
	if abortErr := t.s.reader.AbortStart(t.handle); abortErr != nil {
		err = errors.Join(err, abortErr)
	}
	t.phase = startTransactionDone
	return err
}

func (t *sessionStartTransaction) restoreStartState() {
	t.s.doc.identity.abortStart()
	t.s.doc.schemaLocationHints = t.hints
	clear(t.s.doc.allBits[t.allBitsLen:])
	t.s.doc.allBits = t.s.doc.allBits[:t.allBitsLen]
}

func (t *sessionStartTransaction) restoreRecoveryState() {
	clear(t.s.doc.errors[t.errorsLen:])
	t.s.doc.errors = t.s.doc.errors[:t.errorsLen]
	t.s.doc.syntaxOnly = t.syntaxOnly
}

func (s *session) start(line, col int, attrs []xmlstream.Attr) error {
	if s.doc.syntaxOnly {
		return s.syntaxStart(line, col)
	}
	xmlCheckpoint := s.doc.startCheckpoint()
	se, err := s.doc.PrepareStart(&s.reader, s.limits, line, col)
	if err != nil {
		return err
	}
	var transaction sessionStartTransaction
	if err := transaction.begin(s, xmlCheckpoint, se.handle); err != nil {
		if abortErr := s.doc.AbortStart(&s.reader, se); abortErr != nil {
			return errors.Join(err, abortErr)
		}
		return err
	}
	resultErr := s.runStartTransaction(&transaction, se, attrs, line, col)
	if abortErr := transaction.abort(); abortErr != nil {
		return errors.Join(resultErr, abortErr)
	}
	return resultErr
}

func (s *session) runStartTransaction(
	transaction *sessionStartTransaction,
	se preparedXMLStart,
	attrs []xmlstream.Attr,
	line, col int,
) error {
	xsiFlags := xsiStartAttributeFlagsFor(attrs)
	// Hint syntax and limits are diagnosed before selection. Hints never
	// influence which declaration or type is selected.
	if xsiFlags.SchemaLocation {
		if err := s.recover(s.recordSchemaLocationHints(attrs, line, col)); err != nil {
			return handleStartTransactionError(transaction, se, err)
		}
	}
	rn := s.runtimeName(se.name)
	var accepted acceptedChild
	err := s.startType(rn, attrs, xsiFlags, line, col, &accepted)
	if err != nil {
		return handleStartTransactionError(transaction, se, err)
	}
	transaction.stageContent(&accepted)
	start := accepted.start
	nilled, err := s.assessElementStart(&start, attrs, xsiFlags)
	if err != nil {
		return handleStartTransactionError(transaction, se, err)
	}
	schemaFrame, err := s.newSchemaFrame(start, nilled)
	if err != nil {
		return err
	}
	if err := transaction.commitXMLStart(se, expandedInstancePath(start.mode, rn), schemaFrame); err != nil {
		return err
	}
	if identityErr := s.startFrameIdentity(rn, &schemaFrame, line, col); identityErr != nil {
		return handleStartTransactionError(transaction, se, identityErr)
	}
	if attrErr := s.validateStartAttributes(start, attrs, line, col); attrErr != nil {
		return handleStartTransactionError(transaction, se, attrErr)
	}
	return transaction.commit()
}

func handleStartTransactionError(transaction *sessionStartTransaction, start preparedXMLStart, err error) error {
	if errors.Is(err, errSemanticStop) {
		return transaction.stopSemanticValidation(start)
	}
	return err
}

func (s *session) assessElementStart(
	start *schemaStart,
	attrs []xmlstream.Attr,
	flags xsiStartAttributeFlags,
) (bool, error) {
	if start.mode != elementAssessed {
		return false, nil
	}
	info, complete, err := s.initialElementAssessment(start, start.declaration)
	if complete {
		return false, err
	}
	// Initial assessment may replace the start, so read its declaration after.
	declaration := start.declaration
	if !flags.Nil && !flags.Type {
		issue := elementEffectiveTypeIssue(start.typ, info)
		if !issue.valid() {
			return false, nil
		}
		err := validationFromIssue(startContext{}, elementStartDiagnosticIssue(s.rt, start, issue))
		return false, s.recoverElementStartAssessment(start, err)
	}
	state := elementEffectiveState{declaration: declaration, typeID: start.typ, typeInfo: info}
	return s.assessXSIElementStart(start, attrs, flags, state)
}

// assessXSIElementStart assesses xsi:nil, then xsi:type. Each keeps its
// successful result when the other fails, and each owns its own diagnostic;
// identity capture of the same attributes never repeats them.
func (s *session) assessXSIElementStart(
	start *schemaStart,
	attrs []xmlstream.Attr,
	flags xsiStartAttributeFlags,
	state elementEffectiveState,
) (bool, error) {
	nilValue, typeValue, err := s.xsiStartValues(attrs)
	if err != nil {
		return false, err
	}
	if flags.Nil {
		var nilValid bool
		state.nil.value, nilValid = parseXSINil(nilValue)
		state.nil.specified = true
		if !nilValid {
			err := xsderrors.Validation(xsderrors.CodeValidationNil, invalidNilMessage(nilValue), nil)
			if err = s.recoverElementStartAssessment(start, err); err != nil {
				return false, err
			}
		}
	}
	if flags.Type {
		info, complete, err := s.assessXSIType(start, xsiTypeAssessment{
			declaration: state.declaration,
			lexical:     typeValue,
			typeInfo:    state.typeInfo,
		})
		if complete {
			return state.nilled(), err
		}
		state.typeID = start.typ
		state.typeInfo = info
	}
	return s.completeElementStartAssessment(start, state)
}

// xsiStartValues materializes the xsi:nil and xsi:type lexical values.
func (s *session) xsiStartValues(attrs []xmlstream.Attr) (nilValue, typeValue string, err error) {
	for i := range attrs {
		a := &attrs[i]
		kind := xsiStartValueFor(a.Name)
		if kind == xsiStartNoValue {
			continue
		}
		value, err := s.attributeValue(a)
		if err != nil {
			return "", "", err
		}
		if kind == xsiStartNilValue {
			nilValue = value
		} else {
			typeValue = value
		}
	}
	return nilValue, typeValue, nil
}

func (s *session) completeElementStartAssessment(start *schemaStart, state elementEffectiveState) (bool, error) {
	issue := state.issue()
	if !issue.valid() {
		return state.nilled(), nil
	}
	err := validationFromIssue(startContext{}, elementStartDiagnosticIssue(s.rt, start, issue))
	return state.nilled(), s.recoverElementStartAssessment(start, err)
}

// expandedInstancePath selects an expanded {namespace}local path segment for an
// assessed element whose namespaced name the schema does not know, so its own
// and its descendants' diagnostics stay unambiguous without prefixes. Known
// names and skipped wildcard content keep lexical segments.
func expandedInstancePath(mode elementMode, rn xsdSchema.RuntimeName) xmlPathMode {
	if mode == elementAssessed && !rn.Known && rn.NS != "" {
		return xmlPathExpanded
	}
	return xmlPathLexical
}

func (s *session) startFrameIdentity(rn xsdSchema.RuntimeName, f *frame, line, col int) error {
	if !s.doc.identity.active() {
		return nil
	}
	return s.doc.identity.startElement(identityElementStart{
		Name:          rn,
		Element:       f.Element,
		Mode:          f.Mode,
		Context:       s.startContext(line, col),
		Nilled:        f.Nilled,
		SimpleContent: f.SimpleContent != xsdSchema.NoSimpleType,
	})
}

func (s *session) validateStartAttributes(start schemaStart, attrs []xmlstream.Attr, line, col int) error {
	switch start.mode {
	case elementAssessed:
		return s.validateAttributes(start.typ, attrs, line, col)
	case elementWildcardSkipped:
		return nil
	default:
		return xsderrors.InternalInvariant("element assessment mode is invalid")
	}
}

func (s *session) syntaxStart(line, col int) error {
	start, err := s.doc.PrepareStart(&s.reader, s.limits, line, col)
	if err != nil {
		return err
	}
	s.doc.CommitStart(start, frame{})
	return nil
}

func (s *session) initialElementAssessment(start *schemaStart, declaration startDeclaration) (xsdSchema.TypeInfo, bool, error) {
	info, known := s.rt.TypeInfo(start.typ)
	if !known {
		return xsdSchema.TypeInfo{}, true, xsderrors.InternalInvariant("start type metadata is invalid")
	}
	if !info.Unavailable {
		if declaration.present && declaration.abstract {
			// An abstract declaration still governs: its type assesses the content.
			err := xsderrors.Validation(xsderrors.CodeValidationElement, abstractElementMessage(s.rt, start.element), nil)
			if err = s.recoverElementStartAssessment(start, err); err != nil {
				return xsdSchema.TypeInfo{}, true, err
			}
		}
		return info, false, nil
	}
	// An unavailable type acts as if the declaration were absent (Structures
	// §5.3, cvc-elt clause 1), so its abstract flag, nillability, value
	// constraint, and identity constraints no longer apply and the element is
	// assessed laxly as anyType. A present xsi:type is then resolved against
	// anyType and assesses the element when it resolves. The type origin is
	// kept so a root xsi:type is not resolved again.
	message := unavailableElementTypeMessage(s.rt, start.typ)
	*start = schemaStart{element: xsdSchema.NoElement, typ: s.rt.AnyType(), mode: elementAssessed, typeOrigin: start.typeOrigin}
	err := xsderrors.Validation(xsderrors.CodeValidationElement, message, nil)
	if err = s.recoverElementStartAssessment(start, err); err != nil {
		return xsdSchema.TypeInfo{}, true, err
	}
	info, known = s.rt.TypeInfo(start.typ)
	if !known {
		return xsdSchema.TypeInfo{}, true, xsderrors.InternalInvariant("anyType metadata is invalid")
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

type xsiTypeAssessment struct {
	lexical     string
	declaration startDeclaration
	typeInfo    xsdSchema.TypeInfo
}

func (s *session) assessXSIType(start *schemaStart, assessment xsiTypeAssessment) (xsdSchema.TypeInfo, bool, error) {
	override := start.typ
	if start.typeOrigin == selectedTypeRootXSI {
		start.typeOrigin = selectedTypeDefault
	} else {
		var err error
		override, err = resolveXSIType(s.rt, assessment.lexical, s.qnameResolver())
		if err != nil {
			return s.recoverXSITypeError(start, assessment.typeInfo, err)
		}
	}
	overrideInfo, known := s.rt.TypeInfo(override)
	if !known {
		return assessment.typeInfo, true, xsderrors.InternalInvariant("start type metadata is invalid")
	}
	if overrideInfo.Unavailable {
		// Like any rejected override, the governing type remains selected.
		err := xsderrors.Validation(xsderrors.CodeValidationElement, unavailableElementTypeMessage(s.rt, override), nil)
		return s.recoverXSITypeError(start, assessment.typeInfo, err)
	}
	if err := validateXSITypeOverride(s.rt, &s.derivationScratch, xsiTypeOverrideInput{
		declaration:   assessment.declaration,
		declared:      start.typ,
		declaredBlock: assessment.typeInfo.Block,
		override:      override,
	}); err != nil {
		return s.recoverXSITypeError(start, assessment.typeInfo, err)
	}
	start.typ = override
	return overrideInfo, false, nil
}

func (s *session) recoverXSITypeError(start *schemaStart, info xsdSchema.TypeInfo, err error) (xsdSchema.TypeInfo, bool, error) {
	err = s.recoverElementStartAssessment(start, err)
	return info, err != nil, err
}

// recoverElementStartAssessment is the location boundary for start assessment,
// which returns unlocated failures: it marks the start invalid and attaches the
// admitted start's path and token position before recording the diagnostic.
func (s *session) recoverElementStartAssessment(start *schemaStart, err error) error {
	if assessmentFailure(err) {
		start.invalid = true
		err = s.locateElementStartError(err)
	}
	return s.recover(err)
}

// selectedTypeOrigin is a transient origin on the canonical selected start
// state, so root selection does not resolve xsi:type twice. Rejected: a
// separate root-start flag result, because its sole consumer immediately
// translated it into the selected state; and resolving all XSI types eagerly,
// because undeclared-root selection and declared-start nil/type assessment
// have different diagnostic precedence.
type selectedTypeOrigin uint8

const (
	selectedTypeDefault selectedTypeOrigin = iota
	selectedTypeRootXSI
)

type schemaStart struct {
	typ         xsdSchema.TypeID
	element     xsdSchema.ElementID
	declaration startDeclaration
	mode        elementMode
	typeOrigin  selectedTypeOrigin
	invalid     bool
}

func undeclaredSchemaStart(typ xsdSchema.TypeID) schemaStart {
	return schemaStart{element: xsdSchema.NoElement, typ: typ, mode: elementAssessed}
}

func declaredSchemaStart(element xsdSchema.ElementID, declaration xsdSchema.ElementStartInfo) schemaStart {
	return schemaStart{
		element: element,
		typ:     declaration.Type,
		mode:    elementAssessed,
		declaration: startDeclaration{
			present:  true,
			block:    declaration.Block,
			abstract: declaration.Abstract,
			nillable: declaration.Nillable,
			fixed:    declaration.Fixed,
		},
	}
}

// Root selection must resolve xsi:type before common nil/type assessment.
// Its origin is consumed during that start and never retained in a frame.
func rootXSITypeStart(typ xsdSchema.TypeID) schemaStart {
	return schemaStart{element: xsdSchema.NoElement, typ: typ, mode: elementAssessed, typeOrigin: selectedTypeRootXSI}
}

func wildcardSkippedSchemaStart() schemaStart {
	return schemaStart{element: xsdSchema.NoElement, mode: elementWildcardSkipped}
}

// accepted is caller-owned and is consumed only after this method succeeds.
// Child recovery may fill a complete recovery result before returning an error.
func (s *session) startType(rn xsdSchema.RuntimeName, attrs []xmlstream.Attr, flags xsiStartAttributeFlags, line, col int, accepted *acceptedChild) error {
	if s.doc.Depth() == 0 {
		start, err := s.rootStartType(rn, attrs, line, col)
		if err != nil {
			return err
		}
		*accepted = acceptedChild{start: start}
		return nil
	}
	parent, ok := s.doc.Current()
	if !ok {
		return xsderrors.InternalInvariant("child start has no parent frame")
	}
	err := s.acceptChild(parent, rn, flags, line, col, accepted)
	if err == nil {
		return nil
	}
	invalidatesParent := assessmentFailure(err)
	if recoverErr := s.recover(err); recoverErr != nil {
		return recoverErr
	}
	accepted.invalidatesParent = invalidatesParent
	return nil
}

// xsiTypeIndex returns the index of the xsi:type attribute, or -1.
func xsiTypeIndex(attrs []xmlstream.Attr) int {
	for i := range attrs {
		if isXSITypeName(attrs[i].Name) {
			return i
		}
	}
	return -1
}

func (s *session) rootStartType(rn xsdSchema.RuntimeName, attrs []xmlstream.Attr, line, col int) (schemaStart, error) {
	if id, decl, ok := s.rt.GlobalElement(rn); ok {
		return declaredSchemaStart(id, decl), nil
	}
	ctx := s.startContext(line, col)
	if i := xsiTypeIndex(attrs); i >= 0 {
		value, valueErr := s.attributeValue(&attrs[i])
		if valueErr != nil {
			return schemaStart{}, valueErr
		}
		typ, err := resolveXSIType(s.rt, value, s.qnameResolver())
		if err != nil {
			// The failed hint is consumed here; anyType assesses the root laxly.
			start := rootXSITypeStart(s.rt.AnyType())
			if recoverErr := s.recoverElementStartAssessment(&start, err); recoverErr != nil {
				return schemaStart{}, recoverErr
			}
			return start, nil
		}
		return rootXSITypeStart(typ), nil
	}
	err := validation(ctx, xsderrors.CodeValidationRoot, undeclaredRootMessage(xml.Name{Space: rn.NS, Local: rn.Local}))
	if recoverErr := s.recover(err); recoverErr != nil {
		return schemaStart{}, recoverErr
	}
	start := undeclaredSchemaStart(s.rt.AnyType())
	start.invalid = true
	return start, nil
}

func (s *session) startContext(line, col int) startContext {
	return s.doc.context(line, col)
}

func (s *session) newSchemaFrame(
	start schemaStart,
	nilled bool,
) (frame, error) {
	if start.mode != elementAssessed {
		return frame{
			Element:       xsdSchema.NoElement,
			SimpleContent: xsdSchema.NoSimpleType,
			BitBase:       len(s.doc.allBits),
			TextStart:     len(s.doc.text),
			Mode:          start.mode,
		}, nil
	}
	elem := start.element
	typ := start.typ
	read, ok := s.rt.ElementFrame(typ, elem)
	if !ok {
		return frame{}, xsderrors.InternalInvariant("element frame metadata is invalid")
	}
	contentFrame := read.Content
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
		TextContent:       read.TextContent,
		SimpleContent:     read.SimpleContent,
		TextStart:         len(s.doc.text),
		Nilled:            nilled,
		Mode:              elementAssessed,
		AssessmentInvalid: start.invalid,
	}, nil
}

func (s *session) chars(line, col int, data []byte) error {
	if s.doc.syntaxOnly && s.doc.Depth() != 0 {
		return nil
	}
	f, ok := s.doc.Current()
	if !ok {
		// Reader.Next admits only literal whitespace outside the document
		// element; all other outside-root data is returned as a translated
		// stream boundary error before this consumer sees a token.
		return nil
	}
	if len(data) == 0 || f.Mode != elementAssessed {
		return nil
	}
	if f.Nilled {
		return validation(s.startContext(line, col), xsderrors.CodeValidationNil, msgNilledContent)
	}
	return s.validateAssessedCharacterData(f, data, line, col)
}

// validateAssessedCharacterData applies the content type's character policy.
// Empty content rejects any nonempty character data, including XML
// whitespace; element-only content permits whitespace only. Comments, PIs and
// zero-length CDATA never reach here, so they add no character content.
func (s *session) validateAssessedCharacterData(f *frame, data []byte, line, col int) error {
	if f.SimpleContent != xsdSchema.NoSimpleType {
		return s.appendText(data, line, col)
	}
	content := f.TextContent
	if content.IsEmptyContent() {
		return validation(s.startContext(line, col), xsderrors.CodeValidationText, msgEmptyText)
	}
	whitespace := lex.IsXMLWhitespaceBytes(data)
	if !whitespace {
		f.HasText = true
	}
	if content.AllowsMixedContent() {
		return s.captureMixedCharacterData(content, data, line, col)
	}
	if !whitespace {
		ctx := s.startContext(line, col)
		return validation(ctx, xsderrors.CodeValidationText, msgElementOnlyText)
	}
	return nil
}

func (s *session) captureMixedCharacterData(content xsdSchema.ElementTextContent, data []byte, line, col int) error {
	if content.HasFixedElementValue() {
		return s.appendText(data, line, col)
	}
	return nil
}

func (s *session) appendText(data []byte, line, col int) error {
	if s.limits.InstanceTextBytes > 0 && int64(len(s.doc.text)) > s.limits.InstanceTextBytes-int64(len(data)) {
		return validation(s.startContext(line, col), xsderrors.CodeValidationLimit, limitMessage("instance text byte", "MaxInstanceTextBytes", int64(len(s.doc.text))+int64(len(data)), s.limits.InstanceTextBytes))
	}
	s.doc.text = append(s.doc.text, data...)
	return nil
}
