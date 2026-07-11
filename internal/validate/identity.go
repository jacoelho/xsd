package validate

import (
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// IdentityLimits bounds retained identity values while validating a document.
type IdentityLimits struct {
	Entries    int
	TupleBytes int64
}

// IdentityValue is the ID/IDREF projection of a validated simple value.
type IdentityValue struct {
	IDs    string
	IDRefs string
}

const nilledElementIdentityKey = "\xff\x1e\x00nil"

// NilledElementIdentityKey returns the identity-field key used for selected
// nilled elements.
func NilledElementIdentityKey() string {
	return nilledElementIdentityKey
}

// EndIdentityCaptureAction identifies the identity field capture needed after
// element content validation.
type EndIdentityCaptureAction uint8

const (
	// EndIdentityCaptureNone means end-of-element handling has no identity value.
	EndIdentityCaptureNone EndIdentityCaptureAction = iota
	// EndIdentityCaptureNilledElement means selected fields use the nilled sentinel.
	EndIdentityCaptureNilledElement
	// EndIdentityCaptureComplexElement means selected fields use element text.
	EndIdentityCaptureComplexElement
)

// EndIdentityCapture selects the identity capture action for an element end.
func EndIdentityCapture(rt EndIdentityRuntime, in EndIdentityInput) (EndIdentityCaptureAction, error) {
	if in.ContentCaptured {
		return EndIdentityCaptureNone, nil
	}
	if rt == nil {
		return EndIdentityCaptureNone, xsderrors.InternalInvariant("end identity runtime is missing")
	}
	hasSimpleContent, ok := rt.ElementHasSimpleContent(in.Type, in.Element)
	if !ok {
		return EndIdentityCaptureNone, xsderrors.InternalInvariant("end identity content info is invalid")
	}
	if in.Nilled && in.Element != runtime.NoElement {
		return EndIdentityCaptureNilledElement, nil
	}
	if !hasSimpleContent {
		return EndIdentityCaptureComplexElement, nil
	}
	return EndIdentityCaptureNone, nil
}

// EndIdentityInput is the validation state needed to finish element identity
// field capture after content validation.
type EndIdentityInput struct {
	Type            runtime.TypeID
	Element         runtime.ElementID
	ContentCaptured bool
	Nilled          bool
}

// EndIdentityRuntime supplies element content metadata needed to finish
// identity field capture after content validation.
type EndIdentityRuntime interface {
	ElementHasSimpleContent(typ runtime.TypeID, elem runtime.ElementID) (bool, bool)
}

// SimpleValueIdentityRuntime supplies simple-type facts needed to derive
// identity field keys for values without a precomputed identity payload.
type SimpleValueIdentityRuntime interface {
	SimpleTypePrimitive(id runtime.SimpleTypeID) (runtime.PrimitiveKind, bool)
}

// SimpleValueIdentityKey returns the comparable identity field key for a
// validated simple value.
func SimpleValueIdentityKey(rt SimpleValueIdentityRuntime, value runtime.SimpleValue) (string, bool) {
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

// IdentityState owns document-wide ID/IDREF and key/unique/keyref state.
type IdentityState struct {
	ids         map[string]string
	idrefs      []identityRef
	scopes      []identityScope
	selections  []identitySelection
	fieldValues []identityFieldValue
	matches     []IdentityFieldMatch
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

type identityFieldValue struct {
	value   string
	present bool
}

// IdentityFieldMatch identifies one active identity field selected by element
// or attribute content.
type IdentityFieldMatch struct {
	Selection int
	Field     int
}

// IdentityConstraintRuntime supplies identity-constraint metadata needed to
// finish selected identity tuples.
type IdentityConstraintRuntime interface {
	IdentityConstraintInfo(id runtime.IdentityConstraintID) (runtime.IdentityConstraintInfo, bool)
}

// Reset clears document identity state, retaining bounded map/slice capacity.
func (s *IdentityState) Reset(maxRetainedIDs, maxRetainedSlices int) {
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

// ReserveEntry reserves one identity entry against global identity limits.
func (s *IdentityState) ReserveEntry(key string, limits IdentityLimits, ctx StartContext) error {
	if limits.TupleBytes > 0 && int64(len(key)) > limits.TupleBytes {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity tuple byte limit exceeded")
	}
	if limits.Entries > 0 && s.entries >= limits.Entries {
		return validation(ctx, xsderrors.CodeValidationLimit, "identity entry limit exceeded")
	}
	s.entries++
	return nil
}

// CheckIDRefs reports unresolved IDREFs through report.
func (s *IdentityState) CheckIDRefs(report func(error) error) error {
	if s == nil || len(s.idrefs) == 0 {
		return nil
	}
	for _, ref := range s.idrefs {
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

func (s *IdentityState) startScope(constraints runtime.IdentityConstraintIDs, depth int, maxScopes int, ctx StartContext) error {
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
func (s *IdentityState) startElementScope(rt identityScopeRuntime, elem runtime.ElementID, depth int, maxScopes int, ctx StartContext) error {
	if elem == runtime.NoElement {
		return nil
	}
	constraints, ok := rt.ElementIdentityConstraints(elem)
	if !ok {
		return xsderrors.InternalInvariant("element identity constraint metadata is invalid")
	}
	return s.startScope(constraints, depth, maxScopes, ctx)
}

// HasScopes reports whether any identity scopes are active.
func (s *IdentityState) HasScopes() bool {
	return s != nil && len(s.scopes) != 0
}

// StartSelection starts collecting fields for one matched identity selector.
func (s *IdentityState) StartSelection(scope, depth int, constraint runtime.IdentityConstraintID, fieldCount int, ctx StartContext) {
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
}

// ResetFieldMatches clears the scratch field-match list.
func (s *IdentityState) ResetFieldMatches() {
	if s == nil {
		return
	}
	s.matches = s.matches[:0]
}

// AddFieldMatch records that selection's field matched the current value.
func (s *IdentityState) AddFieldMatch(selection, field int) {
	s.matches = append(s.matches, IdentityFieldMatch{Selection: selection, Field: field})
}

// FieldMatches returns field matches accumulated since ResetFieldMatches.
func (s *IdentityState) FieldMatches() []IdentityFieldMatch {
	if s == nil {
		return nil
	}
	return s.matches
}

// elementFieldMatches returns active identity fields matching the current element.
func (s *IdentityState) elementFieldMatches(rt identityElementFieldRuntime, namePath []runtime.RuntimeName) ([]IdentityFieldMatch, error) {
	s.ResetFieldMatches()
	depth := len(namePath)
	for i := range s.selections {
		sel := &s.selections[i]
		fields, ok := rt.IdentityElementFields(sel.constraint)
		if !ok {
			return nil, xsderrors.InternalInvariant("identity element field metadata is invalid")
		}
		for fieldIndex := range fields.Len() {
			field, fieldOK := fields.At(fieldIndex)
			if !fieldOK {
				return nil, xsderrors.InternalInvariant("identity element field metadata is invalid")
			}
			if identityCompiledFieldPathsMatch(rt, namePath, sel.depth, depth, field) {
				s.AddFieldMatch(i, field.Field())
			}
		}
	}
	return s.FieldMatches(), nil
}

// attributeFieldMatches returns active identity fields matching the current attribute.
func (s *IdentityState) attributeFieldMatches(rt identityAttributeFieldRuntime, namePath []runtime.RuntimeName, name runtime.QName) ([]IdentityFieldMatch, error) {
	s.ResetFieldMatches()
	depth := len(namePath)
	for i := range s.selections {
		sel := &s.selections[i]
		start := len(s.matches)
		fields, ok := rt.IdentityAttributeFields(sel.constraint, name)
		if !ok {
			return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
		}
		for fieldIndex := range fields.Len() {
			field, fieldOK := fields.At(fieldIndex)
			if !fieldOK {
				return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
			}
			if identityCompiledFieldPathsMatch(rt, namePath, sel.depth, depth, field) {
				s.AddFieldMatch(i, field.Field())
			}
		}
		fields, ok = rt.IdentityAttributeWildcardFields(sel.constraint)
		if !ok {
			return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
		}
		for fieldIndex := range fields.Len() {
			field, ok := fields.At(fieldIndex)
			if !ok {
				return nil, xsderrors.InternalInvariant("identity attribute field metadata is invalid")
			}
			if identityMatchExists(s.matches[start:], i, field.Field()) {
				continue
			}
			if identityCompiledAttributeFieldPathsMatch(rt, namePath, sel.depth, depth, name, field) {
				s.AddFieldMatch(i, field.Field())
			}
		}
	}
	return s.FieldMatches(), nil
}

// SelectionPath returns the validation path for selection.
func (s *IdentityState) SelectionPath(selection int) (string, bool) {
	if s == nil || selection < 0 || selection >= len(s.selections) {
		return "", false
	}
	return s.selections[selection].path, true
}

// matchSelectors starts selections whose selectors match the current element.
func (s *IdentityState) matchSelectors(rt identitySelectorRuntime, namePath []runtime.RuntimeName, ctx StartContext) error {
	if !s.HasScopes() {
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
			fieldCount, ok := rt.IdentityFieldCount(id)
			if !ok {
				return xsderrors.InternalInvariant("identity field count metadata is invalid")
			}
			s.StartSelection(scopeIndex, depth, id, fieldCount, ctx)
		}
	}
	return nil
}

func identitySelectorMatches(rt identitySelectorPathRuntime, id runtime.IdentityConstraintID, namePath []runtime.RuntimeName, scopeDepth, currentDepth int) (bool, bool) {
	paths, ok := rt.IdentitySelectorPaths(id)
	if !ok {
		return false, false
	}
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

// CaptureFields records one identity value in all matched fields.
func (s *IdentityState) CaptureFields(matches []IdentityFieldMatch, value string, ctx StartContext) error {
	for _, match := range matches {
		if match.Selection < 0 || match.Selection >= len(s.selections) {
			return xsderrors.InternalInvariant("identity field match references invalid selection")
		}
		sel := &s.selections[match.Selection]
		if match.Field < 0 || match.Field >= sel.fieldLen {
			return xsderrors.InternalInvariant("identity field match references invalid field")
		}
		field := &s.selectionFields(*sel)[match.Field]
		if field.present {
			return validation(StartContext{Path: sel.path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "identity field selects multiple values")
		}
		field.value = value
		field.present = true
	}
	return nil
}

// CaptureSimpleValueFields records the identity field key for a selected simple value.
func (s *IdentityState) CaptureSimpleValueFields(rt SimpleValueIdentityRuntime, matches []IdentityFieldMatch, value runtime.SimpleValue, ctx StartContext) error {
	if len(matches) == 0 {
		return nil
	}
	key, ok := SimpleValueIdentityKey(rt, value)
	if !ok {
		return xsderrors.InternalInvariant("identity field value references invalid simple type")
	}
	return s.CaptureFields(matches, key, ctx)
}

// CaptureComplexElementFields records the string identity key for selected
// complex element text, or reports the selected path when no simple value exists.
func (s *IdentityState) CaptureComplexElementFields(matches []IdentityFieldMatch, rawText []byte, ctx StartContext) error {
	if len(matches) == 0 {
		return nil
	}
	if lex.IsXMLWhitespaceBytes(rawText) {
		path, ok := s.SelectionPath(matches[0].Selection)
		if !ok {
			return xsderrors.InternalInvariant("identity field match references invalid selection")
		}
		return validation(StartContext{Path: path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "identity field has no simple value")
	}
	key := runtime.SimpleIdentityKey(runtime.PrimitiveString, lex.CollapseXMLWhitespace(string(rawText)))
	return s.CaptureFields(matches, key, ctx)
}

func (s *IdentityState) finishSelection(
	rt IdentityConstraintRuntime,
	sel identitySelection,
	limits IdentityLimits,
	ctx StartContext,
) error {
	info, ok := rt.IdentityConstraintInfo(sel.constraint)
	if !ok {
		return xsderrors.InternalInvariant("identity constraint metadata is invalid")
	}
	fields := s.selectionFields(sel)
	for _, field := range fields {
		if !field.present {
			if info.Kind == runtime.IdentityKey {
				return validation(StartContext{Path: sel.path, Line: ctx.Line, Column: ctx.Column}, xsderrors.CodeValidationIdentity, "key field is missing")
			}
			return nil
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
	switch info.Kind {
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
		if err := s.ReserveEntry(key, limits, ctx); err != nil {
			return err
		}
		table[key] = identityTableEntry{path: sel.path, node: sel.node}
	case runtime.IdentityKeyRef:
		if err := s.ReserveEntry(key, limits, ctx); err != nil {
			return err
		}
		scope.refs = append(scope.refs, identityTupleRef{
			refer: info.Refer,
			key:   key,
			path:  sel.path,
			line:  sel.line,
			col:   sel.col,
		})
	}
	return nil
}

func (s *IdentityState) selectionFields(sel identitySelection) []identityFieldValue {
	return s.fieldValues[sel.fieldStart : sel.fieldStart+sel.fieldLen]
}

func (s *IdentityState) truncateFieldValues() {
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

func identityTupleKey(fields []identityFieldValue, limits IdentityLimits, ctx StartContext) (string, error) {
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

// CloseScopes closes identity scopes at depth and resolves keyrefs.
func (s *IdentityState) CloseScopes(depth int, report func(error) error) error {
	if s == nil {
		return nil
	}
	for len(s.scopes) > 0 && s.scopes[len(s.scopes)-1].depth == depth {
		scope := &s.scopes[len(s.scopes)-1]
		for _, ref := range scope.refs {
			entry, ok := scope.tables[ref.refer][ref.key]
			if !ok || entry.conflict {
				err := validation(StartContext{Path: ref.path, Line: ref.line, Column: ref.col}, xsderrors.CodeValidationIdentity, "keyref does not resolve")
				if recoverErr := report(err); recoverErr != nil {
					return recoverErr
				}
			}
		}
		if len(s.scopes) > 1 {
			mergeIdentityTables(&s.scopes[len(s.scopes)-2], scope)
		}
		*scope = identityScope{}
		s.scopes = s.scopes[:len(s.scopes)-1]
	}
	return nil
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
