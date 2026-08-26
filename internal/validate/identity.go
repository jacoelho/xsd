package validate

import (
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// identityLimits bounds retained identity values while validating a document.
type identityLimits struct {
	Entries    int
	TupleBytes int64
}

const nilledElementIdentityKey = "\xff\x1e\x00nil"

// endIdentityCaptureAction identifies the identity field capture needed after
// element content validation.
type endIdentityCaptureAction uint8

const (
	// endIdentityCaptureNone means end-of-element handling has no identity value.
	endIdentityCaptureNone endIdentityCaptureAction = iota
	// endIdentityCaptureNilledElement means selected fields use the nilled sentinel.
	endIdentityCaptureNilledElement
	// endIdentityCaptureComplexElement means selected fields use element text.
	endIdentityCaptureComplexElement
)

// endIdentityCaptureForElement selects the identity capture action for an element end.
func endIdentityCaptureForElement(rt *runtime.Schema, in endIdentityInput) (endIdentityCaptureAction, error) {
	if in.ContentCaptured {
		return endIdentityCaptureNone, nil
	}
	if rt == nil {
		return endIdentityCaptureNone, xsderrors.InternalInvariant("end identity runtime is missing")
	}
	hasSimpleContent, ok := rt.ElementHasSimpleContent(in.Type, in.Element)
	if !ok {
		return endIdentityCaptureNone, xsderrors.InternalInvariant("end identity content info is invalid")
	}
	return endIdentityCapture(hasSimpleContent, in), nil
}

func endIdentityCapture(hasSimpleContent bool, in endIdentityInput) endIdentityCaptureAction {
	if !hasSimpleContent {
		return endIdentityCaptureComplexElement
	}
	if in.Nilled && in.Element != runtime.NoElement {
		return endIdentityCaptureNilledElement
	}
	return endIdentityCaptureNone
}

// endIdentityInput is the validation state needed to finish element identity
// field capture after content validation.
type endIdentityInput struct {
	Type            runtime.TypeID
	Element         runtime.ElementID
	ContentCaptured bool
	Nilled          bool
}

// simpleValueIdentityKey returns the comparable identity field key for a
// validated simple value.
func simpleValueIdentityKey(rt *runtime.Schema, value runtime.SimpleValue) (string, bool) {
	if value.Identity != "" {
		return value.Identity, true
	}
	if value.Type == runtime.NoSimpleType {
		return runtime.UntypedSimpleIdentityKey(value.Canonical), true
	}
	primitive, ok := rt.SimpleTypePrimitive(value.Type)
	if !ok {
		return "", false
	}
	return runtime.SimpleIdentityKey(primitive, value.Canonical), true
}

// identityState stores the evaluator's document-wide ID/IDREF and
// key/unique/keyref data.
type identityState struct {
	ids         map[string]string
	idrefs      []identityRef
	scopes      []identityScope
	selections  []identitySelection
	fieldValues []identityFieldValue
	matches     []identityFieldMatch
	entries     int
	nextNodeID  uint64
}

type identityRef struct {
	Value string
	Path  string
	Line  int
	Col   int
}

type identityScope struct {
	tables      map[runtime.IdentityConstraintID]map[string]identityTableEntry
	constraints runtime.IdentityConstraintIDs
	refs        []identityTupleRef
	depth       int
	invalid     bool
}

// identityTableEntry records where a key tuple was first seen. Conflict marks
// tuples propagated from child scopes with differing selected nodes.
type identityTableEntry struct {
	path     string
	node     uint64
	conflict bool
}

type identityTupleRef struct {
	key   string
	path  string
	line  int
	col   int
	refer runtime.IdentityConstraintID
}

type identitySelection struct {
	path       string
	node       uint64
	scope      int
	depth      int
	fieldStart int
	fieldLen   int
	line       int
	col        int
	constraint runtime.IdentityConstraintID
}

type identityFieldState uint8

const (
	identityFieldAbsent identityFieldState = iota
	identityFieldPresent
	identityFieldInvalid
)

type identityFieldValue struct {
	value    string
	state    identityFieldState
	nillable bool
}

// identityFieldMatch identifies one active identity field selected by element
// or attribute content.
type identityFieldMatch struct {
	Selection int
	Field     int
}

// Reset clears document identity state, retaining bounded map/slice capacity.
func (s *identityState) reset(maxRetainedIDs, maxRetainedSlices int) {
	if s == nil {
		return
	}
	if len(s.ids) > maxRetainedIDs {
		s.ids = nil
	} else {
		clear(s.ids)
	}
	if cap(s.idrefs) > maxRetainedSlices {
		s.idrefs = nil
	} else {
		clear(s.idrefs)
		s.idrefs = s.idrefs[:0]
	}
	s.scopes = resetRetainedReferences(s.scopes, maxRetainedSlices)
	s.selections = resetRetainedReferences(s.selections, maxRetainedSlices)
	s.fieldValues = resetRetainedReferences(s.fieldValues, maxRetainedSlices)
	s.matches = resetRetainedValues(s.matches, maxRetainedSlices)
	s.entries = 0
	s.nextNodeID = 0
}

// reserveEntry reserves one identity entry against global identity limits.
func (s *identityState) reserveEntry(key string, limits identityLimits, ctx StartContext) error {
	if limits.TupleBytes > 0 && int64(len(key)) > limits.TupleBytes {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity tuple byte limit exceeded")
	}
	if limits.Entries > 0 && s.entries >= limits.Entries {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity entry limit exceeded")
	}
	s.entries++
	return nil
}

// checkIDRefs reports unresolved IDREFs through report. When check is non-nil,
// it runs before each retained reference.
func (s *identityState) checkIDRefs(report func(error) error, check func() error) error {
	if s == nil || len(s.idrefs) == 0 {
		return nil
	}
	for _, ref := range s.idrefs {
		if check != nil {
			if err := check(); err != nil {
				return err
			}
		}
		if _, ok := s.ids[ref.Value]; ok {
			continue
		}
		err := validation(StartContext{Path: ref.Path, Line: ref.Line, Column: ref.Col}, xsderrors.CodeValidationType, "IDREF does not resolve: "+ref.Value)
		if recoverErr := report(err); recoverErr != nil {
			return recoverErr
		}
	}
	return nil
}

func (s *identityState) startScope(constraints runtime.IdentityConstraintIDs, depth int, maxScopes int, ctx StartContext) error {
	if constraints.Len() == 0 {
		return nil
	}
	if maxScopes > 0 && len(s.scopes) >= maxScopes {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity scope limit exceeded")
	}
	s.scopes = append(s.scopes, identityScope{
		depth:       depth,
		constraints: constraints,
	})
	return nil
}

// startElementScope starts an identity scope declared on elem.
func (s *identityState) startElementScope(rt *runtime.Schema, elem runtime.ElementID, depth int, maxScopes int, ctx StartContext) error {
	if elem == runtime.NoElement {
		return nil
	}
	constraints, ok := rt.ElementIdentityConstraints(elem)
	if !ok {
		return xsderrors.InternalInvariant("element identity constraint metadata is invalid")
	}
	return s.startScope(constraints, depth, maxScopes, ctx)
}

// startSelection starts collecting fields for one matched identity selector
// after enforcing the active-selection bound at the allocation boundary.
func (s *identityState) startSelection(scope, depth int, constraint runtime.IdentityConstraintID, fieldCount, maxPending int, ctx StartContext) error {
	if maxPending > 0 && len(s.selections) >= maxPending {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity entry limit exceeded")
	}
	fieldStart := len(s.fieldValues)
	for range fieldCount {
		s.fieldValues = append(s.fieldValues, identityFieldValue{})
	}
	s.nextNodeID++
	s.selections = append(s.selections, identitySelection{
		scope:      scope,
		constraint: constraint,
		depth:      depth,
		fieldStart: fieldStart,
		fieldLen:   fieldCount,
		path:       ctx.PathString(),
		node:       s.nextNodeID,
		line:       ctx.Line,
		col:        ctx.Column,
	})
	return nil
}

// elementFieldMatches returns active identity fields matching the current element.
func (s *identityState) elementFieldMatches(rt *runtime.Schema, namePath []runtime.RuntimeName) ([]identityFieldMatch, error) {
	s.matches = s.matches[:0]
	depth := len(namePath)
	for i := range s.selections {
		sel := &s.selections[i]
		constraint, ok := rt.IdentityConstraint(sel.constraint)
		if !ok {
			return nil, xsderrors.InternalInvariant("identity element field metadata is invalid")
		}
		fields := constraint.ElementFields()
		for fieldIndex := range fields.Len() {
			field, fieldOK := fields.At(fieldIndex)
			if !fieldOK {
				return nil, xsderrors.InternalInvariant("identity element field metadata is invalid")
			}
			if identityCompiledFieldPathsMatch(rt, namePath, sel.depth, depth, field) {
				s.matches = append(s.matches, identityFieldMatch{Selection: i, Field: field.Field()})
			}
		}
	}
	return s.matches, nil
}

// attributeFieldMatches returns active identity fields matching the current attribute.
func (s *identityState) attributeFieldMatches(rt *runtime.Schema, namePath []runtime.RuntimeName, name runtime.RuntimeName) ([]identityFieldMatch, error) {
	s.matches = s.matches[:0]
	depth := len(namePath)
	for i := range s.selections {
		sel := &s.selections[i]
		start := len(s.matches)
		constraint, ok := rt.IdentityConstraint(sel.constraint)
		if !ok {
			return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
		}
		if name.Known {
			fields := constraint.AttributeFields(name.Name)
			for fieldIndex := range fields.Len() {
				field, fieldOK := fields.At(fieldIndex)
				if !fieldOK {
					return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
				}
				if identityCompiledFieldPathsMatch(rt, namePath, sel.depth, depth, field) {
					s.matches = append(s.matches, identityFieldMatch{Selection: i, Field: field.Field()})
				}
			}
		}
		fields := constraint.AttributeWildcardFields()
		for fieldIndex := range fields.Len() {
			field, ok := fields.At(fieldIndex)
			if !ok {
				return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
			}
			if identityMatchExists(s.matches[start:], i, field.Field()) {
				continue
			}
			if identityCompiledAttributeFieldPathsMatch(rt, namePath, sel.depth, depth, name, field) {
				s.matches = append(s.matches, identityFieldMatch{Selection: i, Field: field.Field()})
			}
		}
	}
	return s.matches, nil
}

// matchSelectors starts selections whose selectors match the current element.
func (s *identityState) matchSelectors(rt *runtime.Schema, namePath []runtime.RuntimeName, maxPending int, ctx StartContext) error {
	if len(s.scopes) == 0 {
		return nil
	}
	depth := len(namePath)
	for scopeIndex := range s.scopes {
		scope := &s.scopes[scopeIndex]
		for constraintIndex := range scope.constraints.Len() {
			id, ok := scope.constraints.At(constraintIndex)
			if !ok {
				return xsderrors.InternalInvariant("identity scope metadata is invalid")
			}
			matched, ok := identitySelectorMatches(rt, id, namePath, scope.depth, depth)
			if !ok {
				return xsderrors.InternalInvariant("identity selector metadata is invalid")
			}
			if !matched {
				continue
			}
			constraint, ok := rt.IdentityConstraint(id)
			if !ok {
				return xsderrors.InternalInvariant("identity field count metadata is invalid")
			}
			if err := s.startSelection(scopeIndex, depth, id, constraint.FieldCount(), maxPending, ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func identitySelectorMatches(rt *runtime.Schema, id runtime.IdentityConstraintID, namePath []runtime.RuntimeName, scopeDepth, currentDepth int) (bool, bool) {
	constraint, ok := rt.IdentityConstraint(id)
	if !ok {
		return false, false
	}
	paths := constraint.SelectorPaths()
	for pathIndex := range paths.Len() {
		path, ok := paths.At(pathIndex)
		if !ok {
			return false, false
		}
		if identityPathMatches(rt, namePath, scopeDepth, currentDepth, path) {
			return true, true
		}
	}
	return false, true
}

// captureFields records one identity value in all matched fields.
func (s *identityState) captureFields(matches []identityFieldMatch, value string, ctx StartContext) error {
	if err := s.validateFieldMatches(matches); err != nil {
		return err
	}
	var duplicatePath string
	for _, match := range matches {
		sel := &s.selections[match.Selection]
		field := &s.selectionFields(*sel)[match.Field]
		switch field.state {
		case identityFieldAbsent:
			field.value = value
			field.state = identityFieldPresent
		case identityFieldPresent:
			field.value = ""
			field.state = identityFieldInvalid
			s.scopes[sel.scope].invalid = true
			if duplicatePath == "" {
				duplicatePath = sel.path
			}
		case identityFieldInvalid:
		default:
			return xsderrors.InternalInvariant("identity field state is invalid")
		}
	}
	if duplicatePath != "" {
		return validation(StartContext{Path: duplicatePath, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "identity field selects multiple values")
	}
	return nil
}

// markNillableKeyFields records successfully captured key fields selected from
// nillable element declarations. The rule is enforced only after the complete
// key sequence is known to be qualified.
func (s *identityState) markNillableKeyFields(rt *runtime.Schema, matches []identityFieldMatch) error {
	if len(matches) == 0 {
		return nil
	}
	if rt == nil {
		return xsderrors.InternalInvariant("identity runtime is missing")
	}
	if err := s.validateFieldMatches(matches); err != nil {
		return err
	}
	for _, match := range matches {
		sel := &s.selections[match.Selection]
		constraint, ok := rt.IdentityConstraint(sel.constraint)
		if !ok {
			return xsderrors.InternalInvariant("identity constraint metadata is invalid")
		}
		if constraint.Kind() != runtime.IdentityKey {
			continue
		}
		field := &s.selectionFields(*sel)[match.Field]
		if field.state == identityFieldPresent {
			field.nillable = true
		}
	}
	return nil
}

// rejectFieldsWithoutSimpleValue invalidates selected field nodes that have no
// assessment-derived simple value.
func (s *identityState) rejectFieldsWithoutSimpleValue(matches []identityFieldMatch, ctx StartContext) error {
	if len(matches) == 0 {
		return nil
	}
	if err := s.invalidateFields(matches); err != nil {
		return err
	}
	path := s.selections[matches[0].Selection].path
	return validation(StartContext{Path: path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "identity field has no simple value")
}

// invalidateFields prevents selected field nodes from being reclassified as
// absent or published as identity tuples after another validation failure.
func (s *identityState) invalidateFields(matches []identityFieldMatch) error {
	if err := s.validateFieldMatches(matches); err != nil {
		return err
	}
	for _, match := range matches {
		sel := &s.selections[match.Selection]
		field := &s.selectionFields(*sel)[match.Field]
		field.value = ""
		field.state = identityFieldInvalid
		s.scopes[sel.scope].invalid = true
	}
	return nil
}

func (s *identityState) validateFieldMatches(matches []identityFieldMatch) error {
	for _, match := range matches {
		if match.Selection < 0 || match.Selection >= len(s.selections) {
			return xsderrors.InternalInvariant("identity field match references invalid selection")
		}
		sel := &s.selections[match.Selection]
		if sel.scope < 0 || sel.scope >= len(s.scopes) {
			return xsderrors.InternalInvariant("identity selection references invalid scope")
		}
		if match.Field < 0 || match.Field >= sel.fieldLen {
			return xsderrors.InternalInvariant("identity field match references invalid field")
		}
	}
	return nil
}

func (s *identityState) selectionOwnedAtDepth(sel identitySelection, depth int) (bool, error) {
	if sel.scope < 0 || sel.scope >= len(s.scopes) {
		return false, xsderrors.InternalInvariant("identity selection references invalid scope")
	}
	return s.scopes[sel.scope].depth == depth, nil
}

func (s *identityState) invalidateSelectionScope(sel identitySelection) error {
	if sel.scope < 0 || sel.scope >= len(s.scopes) {
		return xsderrors.InternalInvariant("identity selection references invalid scope")
	}
	s.scopes[sel.scope].invalid = true
	return nil
}

func (s *identityState) finishSelection(
	rt *runtime.Schema,
	sel identitySelection,
	limits identityLimits,
	ctx StartContext,
) error {
	constraint, ok := rt.IdentityConstraint(sel.constraint)
	if !ok {
		return xsderrors.InternalInvariant("identity constraint metadata is invalid")
	}
	return s.finishSelectionWithConstraint(constraint.Kind(), constraint.Refer(), sel, limits, ctx)
}

func (s *identityState) finishSelectionWithConstraint(
	kind runtime.IdentityKind,
	refer runtime.IdentityConstraintID,
	sel identitySelection,
	limits identityLimits,
	ctx StartContext,
) error {
	fields := s.selectionFields(sel)
	invalid := false
	absent := false
	for _, field := range fields {
		switch field.state {
		case identityFieldAbsent:
			absent = true
		case identityFieldPresent:
		case identityFieldInvalid:
			invalid = true
		default:
			return xsderrors.InternalInvariant("identity field state is invalid")
		}
	}
	if invalid {
		return nil
	}
	if absent {
		if kind == runtime.IdentityKey {
			return validation(StartContext{Path: sel.path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "key field is missing")
		}
		return nil
	}
	if kind == runtime.IdentityKey {
		for _, field := range fields {
			if field.nillable {
				return validation(StartContext{Path: sel.path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "key field selects nillable element declaration")
			}
		}
	}
	key, err := identityTupleKey(fields, limits, ctx)
	if err != nil {
		return err
	}
	if sel.scope < 0 || sel.scope >= len(s.scopes) {
		return xsderrors.InternalInvariant("identity selection references invalid scope")
	}
	scope := &s.scopes[sel.scope]
	switch kind {
	case runtime.IdentityUnique, runtime.IdentityKey:
		if scope.tables == nil {
			scope.tables = make(map[runtime.IdentityConstraintID]map[string]identityTableEntry)
		}
		table := scope.tables[sel.constraint]
		if table == nil {
			table = make(map[string]identityTableEntry)
			scope.tables[sel.constraint] = table
		}
		if prev, exists := table[key]; exists {
			return validation(StartContext{Path: sel.path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "duplicate identity value first seen at "+prev.path)
		}
		if err := s.reserveEntry(key, limits, ctx); err != nil {
			return err
		}
		table[key] = identityTableEntry{path: sel.path, node: sel.node}
	case runtime.IdentityKeyRef:
		if err := s.reserveEntry(key, limits, ctx); err != nil {
			return err
		}
		scope.refs = append(scope.refs, identityTupleRef{
			refer: refer,
			key:   key,
			path:  sel.path,
			line:  sel.line,
			col:   sel.col,
		})
	}
	return nil
}

func (s *identityState) selectionFields(sel identitySelection) []identityFieldValue {
	return s.fieldValues[sel.fieldStart : sel.fieldStart+sel.fieldLen]
}

func (s *identityState) truncateFieldValues() {
	n := 0
	for _, sel := range s.selections {
		end := sel.fieldStart + sel.fieldLen
		if end > n {
			n = end
		}
	}
	clear(s.fieldValues[n:])
	s.fieldValues = s.fieldValues[:n]
}

func identityTupleKey(fields []identityFieldValue, limits identityLimits, ctx StartContext) (string, error) {
	size := int64(0)
	for i, field := range fields {
		if i > 0 {
			size++
		}
		size += int64(len(field.value))
		if limits.TupleBytes > 0 && size > limits.TupleBytes {
			return "", validation(ctx, xsderrors.CodeValidationLimit, "identity tuple byte limit exceeded")
		}
	}
	if len(fields) == 1 {
		return fields[0].value, nil
	}
	var b strings.Builder
	b.Grow(int(size))
	for i, field := range fields {
		if i > 0 {
			b.WriteByte('\x1f')
		}
		b.WriteString(field.value)
	}
	return b.String(), nil
}

// closeScopes closes identity scopes at depth, resolves keyrefs, and reports
// whether constraints owned by the closed scopes failed.
func (s *identityState) closeScopes(depth int, report func(error) error) (bool, error) {
	if s == nil {
		return false, nil
	}
	invalid := false
	for len(s.scopes) > 0 && s.scopes[len(s.scopes)-1].depth == depth {
		scope := &s.scopes[len(s.scopes)-1]
		for _, ref := range scope.refs {
			entry, ok := scope.tables[ref.refer][ref.key]
			if !ok || entry.conflict {
				scope.invalid = true
				err := validation(StartContext{Path: ref.path, Line: ref.line, Column: ref.col}, xsderrors.CodeValidationIdentity, "keyref does not resolve")
				if recoverErr := report(err); recoverErr != nil {
					return true, recoverErr
				}
			}
		}
		invalid = invalid || scope.invalid
		if len(s.scopes) > 1 {
			mergeIdentityTables(&s.scopes[len(s.scopes)-2], scope)
		}
		*scope = identityScope{}
		s.scopes = s.scopes[:len(s.scopes)-1]
	}
	return invalid, nil
}

func mergeIdentityTables(dst, src *identityScope) {
	if len(src.tables) == 0 {
		return
	}
	if dst.tables == nil {
		dst.tables = make(map[runtime.IdentityConstraintID]map[string]identityTableEntry)
	}
	for id, srcTable := range src.tables {
		dstTable := dst.tables[id]
		if dstTable == nil {
			dst.tables[id] = srcTable
			continue
		}
		for key, entry := range srcTable {
			prev, exists := dstTable[key]
			switch {
			case !exists:
				dstTable[key] = entry
			case prev.conflict:
			case entry.conflict || prev.node != entry.node:
				dstTable[key] = identityTableEntry{path: prev.path, node: prev.node, conflict: true}
			}
		}
	}
}
