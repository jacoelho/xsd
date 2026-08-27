package validate

import (
	"encoding/xml"
	"slices"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

type identityValueKind uint8

const (
	identityElementValue identityValueKind = iota + 1
	identityAttributeValue
)

type identityValuePhase uint8

const (
	identityTargetInactive identityValuePhase = iota
	identityTargetPrepared
	identityTargetRecorded
	identityTargetCaptured
)

type identityRejection uint8

const (
	identityInvalidValue identityRejection = iota
	identityMissingSimpleValue
)

type identityValueTarget struct {
	generation uint64
	kind       identityValueKind
	matched    bool
}

func (t identityValueTarget) needsIdentity() bool {
	return t.matched
}

type identityElementStart struct {
	Context       StartContext
	Name          runtime.RuntimeName
	Element       runtime.ElementID
	Mode          elementMode
	Nilled        bool
	SimpleContent bool
}

type identityElementEnd struct {
	Context           StartContext
	ContentCaptured   bool
	AssessmentInvalid bool
}

type identityElementResult struct {
	AssessmentInvalid bool
}

type identityElementState struct {
	element       runtime.ElementID
	mode          elementMode
	nilled        bool
	simpleContent bool
	seenID        bool
}

// identityEvaluation owns all document-local XML and XSD identity state.
type identityEvaluation struct {
	rt        *runtime.Schema
	path      []runtime.RuntimeName
	elements  []identityElementState
	targetKey string
	identityState
	limits             identityLimits
	maxScopes          int
	generation         uint64
	targetKind         identityValueKind
	targetPhase        identityValuePhase
	constraintsEnabled bool
}

func newIdentityEvaluation(rt *runtime.Schema, limits identityLimits, maxScopes int) identityEvaluation {
	return identityEvaluation{
		rt:                 rt,
		limits:             limits,
		maxScopes:          maxScopes,
		constraintsEnabled: rt.HasIdentityConstraints(),
	}
}

func (e *identityEvaluation) hasConstraints() bool {
	return e != nil && e.constraintsEnabled
}

func (e *identityEvaluation) beginStart() error {
	if e.startJournal.active {
		return xsderrors.InternalInvariant("identity start transaction already active")
	}
	if e.targetPhase != identityTargetInactive {
		return xsderrors.InternalInvariant("identity value target remains active at element start")
	}
	j := &e.startJournal
	clear(j.addedIDs)
	clear(j.fieldUndos)
	clear(j.scopeUndos)
	*j = identityStartJournal{
		active:         true,
		pathLen:        len(e.path),
		elementsLen:    len(e.elements),
		idrefsLen:      len(e.idrefs),
		scopesLen:      len(e.scopes),
		selectionsLen:  len(e.selections),
		fieldValuesLen: len(e.fieldValues),
		entries:        e.entries,
		nextNodeID:     e.nextNodeID,
		addedIDs:       j.addedIDs[:0],
		fieldUndos:     j.fieldUndos[:0],
		scopeUndos:     j.scopeUndos[:0],
	}
	return nil
}

func (e *identityEvaluation) validateStartCommit() error {
	if !e.startJournal.active {
		return xsderrors.InternalInvariant("identity start transaction is not active")
	}
	if e.targetPhase != identityTargetInactive {
		return xsderrors.InternalInvariant("identity value target remains active at start commit")
	}
	return nil
}

func (e *identityEvaluation) commitStart() {
	e.clearStartJournal()
}

func (e *identityEvaluation) abortStart() {
	j := &e.startJournal
	if !j.active {
		return
	}
	for _, undo := range slices.Backward(j.fieldUndos) {
		e.fieldValues[undo.index] = undo.value
	}
	for _, undo := range slices.Backward(j.scopeUndos) {
		e.scopes[undo.index].invalid = undo.invalid
	}
	for _, id := range j.addedIDs {
		delete(e.ids, id)
	}
	clear(e.idrefs[j.idrefsLen:])
	e.idrefs = e.idrefs[:j.idrefsLen]
	clear(e.scopes[j.scopesLen:])
	e.scopes = e.scopes[:j.scopesLen]
	clear(e.selections[j.selectionsLen:])
	e.selections = e.selections[:j.selectionsLen]
	clear(e.fieldValues[j.fieldValuesLen:])
	e.fieldValues = e.fieldValues[:j.fieldValuesLen]
	clear(e.path[j.pathLen:])
	e.path = e.path[:j.pathLen]
	clear(e.elements[j.elementsLen:])
	e.elements = e.elements[:j.elementsLen]
	e.entries = j.entries
	e.nextNodeID = j.nextNodeID
	e.releaseTarget()
	e.generation++
	e.clearStartJournal()
}

func (e *identityEvaluation) clearStartJournal() {
	j := &e.startJournal
	clear(j.addedIDs)
	clear(j.fieldUndos)
	clear(j.scopeUndos)
	*j = identityStartJournal{
		addedIDs:   j.addedIDs[:0],
		fieldUndos: j.fieldUndos[:0],
		scopeUndos: j.scopeUndos[:0],
	}
}

func (e *identityEvaluation) startElement(in identityElementStart) error {
	if e.targetPhase != identityTargetInactive {
		return xsderrors.InternalInvariant("identity value target remains active at element start")
	}
	switch in.Mode {
	case elementAssessed, elementWildcardSkipped, elementRecovery:
	default:
		return xsderrors.InternalInvariant("element assessment mode is invalid")
	}
	e.elements = append(e.elements, identityElementState{
		element:       in.Element,
		mode:          in.Mode,
		nilled:        in.Nilled,
		simpleContent: in.SimpleContent,
	})
	if !e.constraintsEnabled {
		return nil
	}
	e.path = append(e.path, in.Name)
	if in.Mode == elementRecovery {
		return nil
	}
	depth := len(e.path)
	if err := e.startElementScope(e.rt, in.Element, depth, e.maxScopes, in.Context); err != nil {
		return err
	}
	return e.matchSelectors(e.rt, e.path, e.limits.Entries, in.Context)
}

func (e *identityEvaluation) prepareElementValue() (identityValueTarget, error) {
	return e.prepareValue(identityElementValue, runtime.RuntimeName{})
}

func (e *identityEvaluation) prepareAttributeValue(name runtime.RuntimeName) (identityValueTarget, error) {
	return e.prepareValue(identityAttributeValue, name)
}

func (e *identityEvaluation) prepareValue(kind identityValueKind, name runtime.RuntimeName) (identityValueTarget, error) {
	if e.targetPhase != identityTargetInactive {
		return identityValueTarget{}, xsderrors.InternalInvariant("identity value target already active")
	}
	if len(e.elements) == 0 {
		return identityValueTarget{}, xsderrors.InternalInvariant("identity value has no active element")
	}
	target := identityValueTarget{kind: kind}
	if !e.constraintsEnabled {
		return target, nil
	}
	var (
		matches []identityFieldMatch
		err     error
	)
	switch kind {
	case identityElementValue:
		matches, err = e.elementFieldMatches(e.rt, e.path)
	case identityAttributeValue:
		matches, err = e.attributeFieldMatches(e.rt, e.path, name)
	default:
		return identityValueTarget{}, xsderrors.InternalInvariant("identity value target kind is invalid")
	}
	if err != nil {
		return identityValueTarget{}, err
	}
	if len(matches) == 0 {
		return target, nil
	}
	e.generation++
	if e.generation == 0 {
		e.generation++
	}
	e.targetKind = kind
	e.targetKey = ""
	e.targetPhase = identityTargetPrepared
	target.generation = e.generation
	target.matched = true
	return target, nil
}

func (e *identityEvaluation) recordValue(target identityValueTarget, value runtime.SimpleValue, ctx StartContext) error {
	if err := e.validateTarget(target); err != nil {
		return err
	}
	if target.matched && e.targetPhase != identityTargetPrepared {
		return xsderrors.InternalInvariant("identity value target recorded more than once")
	}
	if target.kind == identityAttributeValue && value.IDs != "" {
		current := &e.elements[len(e.elements)-1]
		if current.seenID {
			err := validation(ctx, xsderrors.CodeValidationType, "multiple ID attributes")
			return e.rejectAfterRecordFailure(target, err)
		}
		current.seenID = true
	}
	if err := e.recordIdentityFields(value.IDs, value.IDRefs, ctx); err != nil {
		return e.rejectAfterRecordFailure(target, err)
	}
	if !target.matched {
		return nil
	}
	key, ok := simpleValueIdentityKey(e.rt, value)
	if !ok {
		e.releaseTarget()
		return xsderrors.InternalInvariant("identity field value references invalid simple type")
	}
	e.targetKey = key
	e.targetPhase = identityTargetRecorded
	return nil
}

func (e *identityEvaluation) captureValue(target identityValueTarget, ctx StartContext) error {
	if err := e.validateTarget(target); err != nil {
		return err
	}
	if !target.matched {
		return nil
	}
	if e.targetPhase == identityTargetPrepared {
		return xsderrors.InternalInvariant("identity value target captured before recording")
	}
	if e.targetPhase == identityTargetCaptured {
		return xsderrors.InternalInvariant("identity value target captured more than once")
	}
	if e.targetPhase != identityTargetRecorded {
		return xsderrors.InternalInvariant("identity value target is not ready for capture")
	}
	if err := e.captureFields(e.matches, e.targetKey, ctx); err != nil {
		e.releaseTarget()
		return err
	}
	e.targetPhase = identityTargetCaptured
	return nil
}

func (e *identityEvaluation) commitValue(target identityValueTarget) error {
	if err := e.validateTarget(target); err != nil {
		return err
	}
	if !target.matched {
		return nil
	}
	if e.targetPhase != identityTargetCaptured {
		return xsderrors.InternalInvariant("identity value target committed before capture")
	}
	e.releaseTarget()
	return nil
}

func (e *identityEvaluation) rejectValue(target identityValueTarget, reason identityRejection, ctx StartContext) error {
	if err := e.validateTarget(target); err != nil {
		return err
	}
	if !target.matched {
		return nil
	}
	defer e.releaseTarget()
	switch reason {
	case identityInvalidValue:
		return e.invalidateFields(e.matches)
	case identityMissingSimpleValue:
		return e.rejectFieldsWithoutSimpleValue(e.matches, ctx)
	default:
		return xsderrors.InternalInvariant("identity value rejection is invalid")
	}
}

func (e *identityEvaluation) captureXSIAttribute(
	target identityValueTarget,
	name xml.Name,
	lexical string,
	resolve runtime.ResolveQNameParts,
	ctx StartContext,
) error {
	if err := e.validateTarget(target); err != nil {
		return err
	}
	if !target.matched {
		return nil
	}
	if e.targetPhase != identityTargetPrepared {
		return xsderrors.InternalInvariant("xsi identity value target is not ready for capture")
	}
	defer e.releaseTarget()
	_, key, ok, err := xsiAttributeIdentityKey(e.rt, name, lexical, resolve, ctx)
	if err != nil {
		if invalidateErr := e.invalidateFields(e.matches); invalidateErr != nil {
			return invalidateErr
		}
		// Start assessment owns XSI diagnostics; this conversion only derives
		// the identity-field key.
		return nil
	}
	if !ok {
		return nil
	}
	return e.captureFields(e.matches, key, ctx)
}

func (e *identityEvaluation) validateTarget(target identityValueTarget) error {
	if !target.matched {
		if e.targetPhase != identityTargetInactive {
			return xsderrors.InternalInvariant("inactive identity value target overlaps an active target")
		}
		if target.kind != identityElementValue && target.kind != identityAttributeValue {
			return xsderrors.InternalInvariant("identity value target kind is invalid")
		}
		return nil
	}
	if e.targetPhase == identityTargetInactive || target.generation == 0 || target.generation != e.generation || target.kind != e.targetKind {
		return xsderrors.InternalInvariant("identity value target is stale")
	}
	return nil
}

func (e *identityEvaluation) rejectAfterRecordFailure(target identityValueTarget, recordErr error) error {
	if !target.matched {
		return recordErr
	}
	invalidateErr := e.invalidateFields(e.matches)
	e.releaseTarget()
	if invalidateErr != nil {
		return invalidateErr
	}
	return recordErr
}

func (e *identityEvaluation) releaseTarget() {
	e.matches = e.matches[:0]
	e.targetKind = 0
	e.targetPhase = identityTargetInactive
	e.targetKey = ""
}

func (e *identityEvaluation) recordIdentityFields(ids, idrefs string, ctx StartContext) error {
	if ids == "" && idrefs == "" {
		return nil
	}
	path := ctx.PathString()
	pendingIDs, err := e.stageIdentityIDs(ids, path, ctx)
	if err != nil {
		return err
	}
	pendingRefs := stageIdentityRefs(idrefs)
	if err := e.validatePendingIdentityFields(pendingIDs, pendingRefs, ctx); err != nil {
		return err
	}
	e.commitIdentityFields(pendingIDs, pendingRefs, path, ctx)
	return nil
}

func (e *identityEvaluation) stageIdentityIDs(ids, path string, ctx StartContext) ([]string, error) {
	pendingIDs := make([]string, 0, 1)
	pendingIDSet := make(map[string]struct{})
	for canonical := range lex.XMLFieldsSeq(ids) {
		if prev, exists := e.ids[canonical]; exists {
			return nil, validation(ctx, xsderrors.CodeValidationType, "duplicate ID "+canonical+" first seen at "+prev)
		}
		if _, exists := pendingIDSet[canonical]; exists {
			return nil, validation(ctx, xsderrors.CodeValidationType, "duplicate ID "+canonical+" first seen at "+path)
		}
		pendingIDSet[canonical] = struct{}{}
		pendingIDs = append(pendingIDs, canonical)
	}
	return pendingIDs, nil
}

func stageIdentityRefs(idrefs string) []string {
	pendingRefs := make([]string, 0, 1)
	for canonical := range lex.XMLFieldsSeq(idrefs) {
		pendingRefs = append(pendingRefs, canonical)
	}
	return pendingRefs
}

func (e *identityEvaluation) validatePendingIdentityFields(ids, refs []string, ctx StartContext) error {
	entryCount := len(ids) + len(refs)
	if e.limits.Entries > 0 && (e.entries > e.limits.Entries || entryCount > e.limits.Entries-e.entries) {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity entry limit exceeded")
	}
	if err := validateIdentityFieldTupleBytes(ids, e.limits.TupleBytes, ctx); err != nil {
		return err
	}
	return validateIdentityFieldTupleBytes(refs, e.limits.TupleBytes, ctx)
}

func validateIdentityFieldTupleBytes(values []string, limit int64, ctx StartContext) error {
	if limit <= 0 {
		return nil
	}
	for _, value := range values {
		if int64(len(value)) > limit {
			return validation(ctx, xsderrors.CodeValidationLimit, "identity tuple byte limit exceeded")
		}
	}
	return nil
}

func (e *identityEvaluation) commitIdentityFields(pendingIDs, pendingRefs []string, path string, ctx StartContext) {
	entryCount := len(pendingIDs) + len(pendingRefs)
	if len(pendingIDs) != 0 && e.ids == nil {
		e.ids = make(map[string]string, len(pendingIDs))
	}
	for _, canonical := range pendingIDs {
		e.rememberAddedID(canonical)
		e.ids[canonical] = path
	}
	for _, canonical := range pendingRefs {
		e.idrefs = append(e.idrefs, identityRef{
			Value: canonical,
			Path:  path,
			Line:  ctx.Line,
			Col:   ctx.Column,
		})
	}
	e.entries += entryCount
}

func (e *identityEvaluation) endElement(in identityElementEnd, report func(error) error) (identityElementResult, error) {
	result := identityElementResult{AssessmentInvalid: in.AssessmentInvalid}
	if e.targetPhase != identityTargetInactive {
		return result, xsderrors.InternalInvariant("identity value target remains active at element end")
	}
	if len(e.elements) == 0 {
		return result, xsderrors.InternalInvariant("identity element stack is empty")
	}
	element := e.elements[len(e.elements)-1]
	defer e.popElement()
	if !e.constraintsEnabled {
		return result, nil
	}
	depth := len(e.path)
	if err := e.finishCurrentIdentityScope(element, in, depth, report); err != nil {
		return result, err
	}
	invalid, err := e.closeScopes(depth, report)
	if err != nil {
		return result, err
	}
	return e.finishAncestorIdentitySelections(result, element, in, depth, invalid, report)
}

func (e *identityEvaluation) finishCurrentIdentityScope(element identityElementState, in identityElementEnd, depth int, report func(error) error) error {
	if err := e.finishElementValue(element, in, report); err != nil {
		return err
	}
	if err := e.finishNillableKeyFields(element, in.AssessmentInvalid); err != nil {
		return err
	}
	return e.finishSelections(depth, in.Context, identitySelectionsOwnedByCurrentScope, report)
}

func (e *identityEvaluation) finishAncestorIdentitySelections(result identityElementResult, element identityElementState, in identityElementEnd, depth int, invalid bool, report func(error) error) (identityElementResult, error) {
	if invalid {
		result.AssessmentInvalid = true
		if err := e.finishNillableKeyFields(element, true); err != nil {
			return result, err
		}
	}
	if err := e.finishSelections(depth, in.Context, identitySelectionsOwnedByAncestorScope, report); err != nil {
		return result, err
	}
	return result, nil
}

func (e *identityEvaluation) finishElementValue(
	element identityElementState,
	in identityElementEnd,
	report func(error) error,
) error {
	switch element.mode {
	case elementWildcardSkipped:
		return e.rejectCurrentElement(identityMissingSimpleValue, in.Context, report)
	case elementRecovery:
		return e.rejectCurrentElement(identityInvalidValue, in.Context, report)
	case elementAssessed:
	default:
		return xsderrors.InternalInvariant("element assessment mode is invalid")
	}
	action := endIdentityCapture(element.simpleContent, endIdentityInput{
		Element:         element.element,
		ContentCaptured: in.ContentCaptured,
		Nilled:          element.nilled,
	})
	switch action {
	case endIdentityCaptureNone:
		return nil
	case endIdentityCaptureNilledElement:
		matches, err := e.elementFieldMatches(e.rt, e.path)
		if err != nil {
			return err
		}
		return reportIdentityError(e.captureFields(matches, nilledElementIdentityKey, in.Context), report)
	case endIdentityCaptureComplexElement:
		return e.rejectCurrentElement(identityMissingSimpleValue, in.Context, report)
	default:
		return xsderrors.InternalInvariant("unknown end identity capture action")
	}
}

func (e *identityEvaluation) rejectCurrentElement(reason identityRejection, ctx StartContext, report func(error) error) error {
	matches, err := e.elementFieldMatches(e.rt, e.path)
	if err != nil {
		return err
	}
	if reason == identityInvalidValue {
		return e.invalidateFields(matches)
	}
	return reportIdentityError(e.rejectFieldsWithoutSimpleValue(matches, ctx), report)
}

func (e *identityEvaluation) finishNillableKeyFields(element identityElementState, invalid bool) error {
	if element.element == runtime.NoElement {
		return nil
	}
	decl, ok := e.rt.Element(element.element)
	if !ok {
		return xsderrors.InternalInvariant("element declaration metadata is invalid")
	}
	if !decl.Nillable {
		return nil
	}
	matches, err := e.elementFieldMatches(e.rt, e.path)
	if err != nil {
		return err
	}
	if invalid {
		return e.invalidateFields(matches)
	}
	return e.markNillableKeyFields(e.rt, matches)
}

type identitySelectionOwnership uint8

const (
	identitySelectionsOwnedByCurrentScope identitySelectionOwnership = iota
	identitySelectionsOwnedByAncestorScope
)

func (e *identityEvaluation) finishSelections(
	depth int,
	ctx StartContext,
	ownership identitySelectionOwnership,
	report func(error) error,
) error {
	if len(e.selections) == 0 {
		return nil
	}
	orig := e.selections
	dst := e.selections[:0]
	for i := range e.selections {
		sel := e.selections[i]
		result, err := e.finishSelectionCandidate(sel, depth, ctx, ownership, report)
		if result.keep {
			dst = append(dst, sel)
		}
		if err != nil {
			remainder := i
			if result.consumed {
				remainder++
			}
			e.restoreSelectionsAfterError(orig, dst, remainder)
			return err
		}
	}
	clear(orig[len(dst):])
	e.selections = dst
	e.truncateFieldValues()
	return nil
}

type identitySelectionFinishResult struct {
	keep     bool
	consumed bool
}

func (e *identityEvaluation) finishSelectionCandidate(sel identitySelection, depth int, ctx StartContext, ownership identitySelectionOwnership, report func(error) error) (identitySelectionFinishResult, error) {
	if sel.depth != depth {
		return identitySelectionFinishResult{keep: true, consumed: true}, nil
	}
	ownedHere, err := e.selectionOwnedAtDepth(sel, depth)
	if err != nil {
		return identitySelectionFinishResult{}, err
	}
	if ownedHere != (ownership == identitySelectionsOwnedByCurrentScope) {
		return identitySelectionFinishResult{keep: true, consumed: true}, nil
	}
	if err := e.finishSelection(e.rt, sel, e.limits, ctx); err != nil {
		return identitySelectionFinishResult{consumed: true}, e.finishSelectionError(sel, err, report)
	}
	clear(e.selectionFields(sel))
	return identitySelectionFinishResult{consumed: true}, nil
}

func (e *identityEvaluation) finishSelectionError(sel identitySelection, err error, report func(error) error) error {
	clear(e.selectionFields(sel))
	if RecoverableError(err) {
		if invalidateErr := e.invalidateSelectionScope(sel); invalidateErr != nil {
			return invalidateErr
		}
	}
	return reportIdentityError(err, report)
}

func (e *identityEvaluation) restoreSelectionsAfterError(orig, dst []identitySelection, remainder int) {
	dst = append(dst, orig[remainder:]...)
	clear(orig[len(dst):])
	e.selections = dst
	e.truncateFieldValues()
}

func reportIdentityError(err error, report func(error) error) error {
	if err == nil {
		return nil
	}
	if report == nil {
		return err
	}
	return report(err)
}

func (e *identityEvaluation) endDocument(report func(error) error) error {
	return e.checkIDRefs(report)
}

func (e *identityEvaluation) popElement() {
	index := len(e.elements) - 1
	e.elements[index] = identityElementState{}
	e.elements = e.elements[:index]
	if !e.constraintsEnabled || len(e.path) == 0 {
		return
	}
	pathIndex := len(e.path) - 1
	e.path[pathIndex] = runtime.RuntimeName{}
	e.path = e.path[:pathIndex]
}

func (e *identityEvaluation) reset(maxRetainedIDs, maxRetainedSlices int) {
	e.identityState.reset(maxRetainedIDs, maxRetainedSlices)
	e.path = resetRetainedReferences(e.path, maxRetainedSlices)
	e.elements = resetRetainedValues(e.elements, maxRetainedSlices)
	e.releaseTarget()
	e.generation++
}

func (e *identityEvaluation) discard() {
	e.identityState = identityState{}
	e.path = nil
	e.elements = nil
	e.releaseTarget()
	e.generation++
}
