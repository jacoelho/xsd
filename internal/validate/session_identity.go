package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func (s *session) validateSimpleContent(f *frame, line, col int) (bool, error) {
	if f.Nilled {
		return false, nil
	}
	typeID := f.SimpleContent
	hasSimpleContent := f.HasSimpleContent
	if !f.SimpleContentKnown {
		var ok bool
		typeID, hasSimpleContent, ok = s.simpleContentType(f.Type)
		if !ok {
			return false, xsderrors.InternalInvariant("simple content type metadata is invalid")
		}
	}
	rawText := s.doc.text[f.TextStart:]
	var constraints runtime.ElementValueConstraints
	declared := f.ElementDeclared
	hasValueConstraint := f.ElementHasValueConstraint
	if !f.ElementValueKnown || hasValueConstraint {
		var ok bool
		constraints, declared, ok = s.elementValueConstraints(f.Element)
		if !ok {
			return false, xsderrors.InternalInvariant("element value constraint metadata is invalid")
		}
		hasValueConstraint = constraints.HasAny()
	}
	if !hasSimpleContent {
		if !hasValueConstraint {
			return false, nil
		}
		ctx := s.startContext(line, col)
		return false, s.validateNonSimpleFixedContent(f, constraints, declared, rawText, ctx)
	}
	var identityFields []IdentityFieldMatch
	if s.hasIdentityConstraints {
		var identityErr error
		identityFields, identityErr = s.identityElementFields()
		if identityErr != nil {
			return false, identityErr
		}
	}
	if len(identityFields) == 0 && !hasValueConstraint {
		ok, rawErr := s.validateRawSimpleValue(typeID, rawText)
		if rawErr != nil {
			if invariantErr := simpleValueMetadataInvariant(rawErr); invariantErr != nil {
				return false, invariantErr
			}
			if ok {
				ctx := s.startContext(line, col)
				return false, validation(ctx, xsderrors.CodeValidationFacet, "invalid simple content: "+rawErr.Error())
			}
			return false, rawErr
		}
		if ok {
			return true, nil
		}
	}
	ctx := s.startContext(line, col)
	input := s.simpleContentValueInput(f.Type, rawText, constraints, declared)
	if input.prevalidated {
		return s.recordElementSimpleContent(input.value, identityFields, ctx, line, col)
	}
	value, err := s.validateSimpleValue(typeID, input.text, s.simpleValueQNameResolver(typeID), s.simpleContentNeeds(typeID, constraints, declared, len(identityFields) != 0))
	if err != nil {
		if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
			return false, invariantErr
		}
		if xsderrors.IsUnsupported(err) {
			return false, err
		}
		return false, validation(ctx, xsderrors.CodeValidationFacet, "invalid simple content: "+err.Error())
	}
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return false, err
	}
	if declared {
		if fixed, ok := constraints.FixedValue(); ok && value.CanonicalText() != fixed.CanonicalText() {
			return false, validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
		}
	}
	if len(identityFields) != 0 {
		if err := s.captureSimpleValueIdentityFields(identityFields, value, ctx); err != nil {
			return false, err
		}
	}
	return true, nil
}

type sessionSimpleContentInput struct {
	text         string
	value        runtime.SimpleValue
	prevalidated bool
}

func (s *session) simpleContentValueInput(
	typ runtime.TypeID,
	rawText []byte,
	constraints runtime.ElementValueConstraints,
	declared bool,
) sessionSimpleContentInput {
	if len(rawText) != 0 || !declared {
		return sessionSimpleContentInput{text: s.valueStrings.Intern(rawText)}
	}
	if fixed, ok := constraints.FixedValue(); ok {
		if constraints.OwnerType() == typ {
			return sessionSimpleContentInput{value: fixed.SimpleValue(), prevalidated: true}
		}
		return sessionSimpleContentInput{text: fixed.LexicalText()}
	}
	if def, ok := constraints.DefaultValueConstraint(); ok {
		if constraints.OwnerType() == typ {
			return sessionSimpleContentInput{value: def.SimpleValue(), prevalidated: true}
		}
		return sessionSimpleContentInput{text: def.LexicalText()}
	}
	return sessionSimpleContentInput{text: s.valueStrings.Intern(rawText)}
}

func (s *session) simpleContentNeeds(
	typeID runtime.SimpleTypeID,
	constraints runtime.ElementValueConstraints,
	declared bool,
	needIdentity bool,
) runtime.SimpleValueNeed {
	var needs runtime.SimpleValueNeed
	if needIdentity {
		needs |= runtime.SimpleNeedIdentity
	}
	if needIdentity || s.simpleIdentity(typeID) != runtime.SimpleIdentityNone {
		needs |= runtime.SimpleNeedCanonical
		return needs
	}
	if declared {
		if _, fixed := constraints.FixedValue(); fixed {
			needs |= runtime.SimpleNeedCanonical
		}
	}
	return needs
}

func (s *session) simpleContentType(typ runtime.TypeID) (runtime.SimpleTypeID, bool, bool) {
	if s.schema != nil {
		return s.schema.SimpleContentType(typ)
	}
	return s.rt.SimpleContentType(typ)
}

func (s *session) elementValueConstraints(id runtime.ElementID) (runtime.ElementValueConstraints, bool, bool) {
	if s.schema != nil {
		return s.schema.ElementValueConstraints(id)
	}
	return s.rt.ElementValueConstraints(id)
}

func (s *session) simpleIdentity(id runtime.SimpleTypeID) runtime.SimpleIdentityKind {
	if s.schema != nil {
		return s.schema.SimpleIdentity(id)
	}
	return s.rt.SimpleIdentity(id)
}

func (s *session) validateNonSimpleFixedContent(
	f *frame,
	constraints runtime.ElementValueConstraints,
	declared bool,
	rawText []byte,
	ctx StartContext,
) error {
	if !declared {
		return nil
	}
	fixed, ok := constraints.FixedValue()
	if !ok {
		return nil
	}
	if f.HasChild {
		return validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	text := s.valueStrings.Intern(rawText)
	if text != "" && text != fixed.LexicalText() {
		return validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	return nil
}

func (s *session) recordElementSimpleContent(
	value runtime.SimpleValue,
	identityFields []IdentityFieldMatch,
	ctx StartContext,
	line, col int,
) (bool, error) {
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return false, err
	}
	if len(identityFields) != 0 {
		if err := s.captureSimpleValueIdentityFields(identityFields, value, ctx); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *session) identityElementFields() ([]IdentityFieldMatch, error) {
	if !s.hasIdentityConstraints {
		return nil, nil
	}
	if s.schema != nil {
		return s.doc.identity.ElementFieldMatchesSchema(s.schema, s.doc.namePath)
	}
	return s.doc.identity.ElementFieldMatches(s.rt, s.doc.namePath)
}

func (s *session) identityAttributeFields(name runtime.QName) ([]IdentityFieldMatch, error) {
	if !s.hasIdentityConstraints {
		return nil, nil
	}
	if s.schema != nil {
		return s.doc.identity.AttributeFieldMatchesSchema(s.schema, s.doc.namePath, name)
	}
	return s.doc.identity.AttributeFieldMatches(s.rt, s.doc.namePath, name)
}

func (s *session) recordAttributeIdentity(value runtime.SimpleValue, line, col int, seenID *bool) error {
	if value.IDs != "" {
		if seenID != nil && *seenID {
			return validation(s.startContext(line, col), xsderrors.CodeValidationType, "multiple ID attributes")
		}
		if seenID != nil {
			*seenID = true
		}
	}
	return s.recordIdentityFields(value.IDs, value.IDRefs, line, col)
}

func (s *session) recordIdentityValue(value runtime.SimpleValue, line, col int) error {
	return s.recordIdentityFields(value.IDs, value.IDRefs, line, col)
}

func (s *session) recordIdentityFields(ids, idrefs string, line, col int) error {
	if ids == "" && idrefs == "" {
		return nil
	}
	path := s.pathString()
	for canonical := range lex.XMLFieldsSeq(ids) {
		if s.doc.identity.ids == nil {
			s.doc.identity.ids = make(map[string]string)
		}
		if prev, exists := s.doc.identity.ids[canonical]; exists {
			return validation(s.startContext(line, col), xsderrors.CodeValidationType, "duplicate ID "+canonical+" first seen at "+prev)
		}
		if err := s.reserveIdentityEntry(canonical, line, col); err != nil {
			return err
		}
		s.doc.identity.ids[canonical] = path
	}
	for canonical := range lex.XMLFieldsSeq(idrefs) {
		if err := s.reserveIdentityEntry(canonical, line, col); err != nil {
			return err
		}
		s.doc.identity.idrefs = append(s.doc.identity.idrefs, identityRef{Value: canonical, Path: path, Line: line, Col: col})
	}
	return nil
}

func (s *session) reserveIdentityEntry(key string, line, col int) error {
	if s.maxIdentityTupleBytes > 0 && int64(len(key)) > s.maxIdentityTupleBytes {
		return validation(s.startContext(line, col), xsderrors.CodeValidationIdentity, "identity tuple byte limit exceeded")
	}
	if s.maxIdentityEntries > 0 && s.doc.identity.entries >= s.maxIdentityEntries {
		return validation(s.startContext(line, col), xsderrors.CodeValidationIdentity, "identity entry limit exceeded")
	}
	s.doc.identity.entries++
	return nil
}

func (s *session) checkIDRefs() error {
	return s.doc.identity.CheckIDRefs(func(err error) error {
		return s.recover(err)
	})
}

func (s *session) startIdentityScope(elem runtime.ElementID, line, col int) error {
	if !s.hasIdentityConstraints {
		return nil
	}
	if s.schema != nil {
		return s.doc.identity.StartElementScopeSchema(s.schema, elem, len(s.doc.namePath), s.maxIdentityScopes, s.startContext(line, col))
	}
	return s.doc.identity.StartElementScope(s.rt, elem, len(s.doc.namePath), s.maxIdentityScopes, s.startContext(line, col))
}

func (s *session) matchIdentitySelectors(line, col int) error {
	if !s.hasIdentityConstraints {
		return nil
	}
	if s.schema != nil {
		return s.doc.identity.MatchSelectorsSchema(s.schema, s.doc.namePath, s.startContext(line, col))
	}
	return s.doc.identity.MatchSelectors(s.rt, s.doc.namePath, s.startContext(line, col))
}

func (s *session) captureIdentityFieldKey(fields []IdentityFieldMatch, key string, line, col int) error {
	if len(fields) == 0 {
		return nil
	}
	return s.doc.identity.CaptureFields(fields, key, s.startContext(line, col))
}

func (s *session) captureSimpleValueIdentityFields(fields []IdentityFieldMatch, value runtime.SimpleValue, ctx StartContext) error {
	if s.schema != nil {
		return s.doc.identity.CaptureSimpleValueFieldsSchema(s.schema, fields, value, ctx)
	}
	return s.doc.identity.CaptureSimpleValueFields(s.rt, fields, value, ctx)
}

func (s *session) captureIdentityXSIAttribute(attrName xml.Name, lexical string, line, col int) error {
	if !s.hasIdentityConstraints {
		return nil
	}
	name, key, ok, err := XSIAttributeIdentityKey(
		s.rt,
		attrName,
		lexical,
		s.resolveLexicalQNamePartsFunc,
		s.startContext(line, col),
	)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	fields, err := s.identityAttributeFields(name)
	if err != nil {
		return err
	}
	return s.captureIdentityFieldKey(fields, key, line, col)
}

func (s *session) captureIdentityComplexElement(rawText []byte, line, col int) error {
	if !s.hasIdentityConstraints {
		return nil
	}
	fields, err := s.identityElementFields()
	if err != nil {
		return err
	}
	return s.doc.identity.CaptureComplexElementFields(fields, rawText, s.startContext(line, col))
}

func (s *session) finishIdentitySelections(depth, line, col int) error {
	if !s.hasIdentityConstraints || len(s.doc.identity.selections) == 0 {
		return nil
	}
	state := &s.doc.identity
	orig := state.selections
	dst := state.selections[:0]
	limits := s.identityLimits()
	ctx := s.startContext(line, col)
	for i := range state.selections {
		sel := state.selections[i]
		if sel.depth != depth {
			dst = append(dst, sel)
			continue
		}
		if err := s.finishIdentitySelection(sel, limits, ctx); err != nil {
			clear(state.selectionFields(sel))
			recoverErr := s.recover(err)
			if recoverErr != nil {
				dst = append(dst, orig[i+1:]...)
				clear(orig[len(dst):])
				state.selections = dst
				state.truncateFieldValues()
				return recoverErr
			}
			continue
		}
		clear(state.selectionFields(sel))
	}
	clear(orig[len(dst):])
	state.selections = dst
	state.truncateFieldValues()
	return nil
}

func (s *session) finishIdentitySelection(sel identitySelection, limits IdentityLimits, ctx StartContext) error {
	info, ok := s.identityConstraintInfo(sel.constraint)
	if !ok {
		return xsderrors.InternalInvariant("identity constraint metadata is invalid")
	}
	fields := s.doc.identity.selectionFields(sel)
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
	if sel.scope < 0 || sel.scope >= len(s.doc.identity.scopes) {
		return xsderrors.InternalInvariant("identity selection references invalid scope")
	}
	scope := &s.doc.identity.scopes[sel.scope]
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
		if err := s.reserveIdentityEntry(key, ctx.Line, ctx.Column); err != nil {
			return err
		}
		table[key] = identityTableEntry{path: sel.path, node: sel.node}
	case runtime.IdentityKeyRef:
		if err := s.reserveIdentityEntry(key, ctx.Line, ctx.Column); err != nil {
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

func (s *session) identityConstraintInfo(id runtime.IdentityConstraintID) (runtime.IdentityConstraintInfo, bool) {
	if s.schema != nil {
		return runtime.IdentityConstraintInfoByID(s.schema.IdentityConstraintReads, id)
	}
	return s.rt.IdentityConstraintInfo(id)
}

func (s *session) identityLimits() IdentityLimits {
	return IdentityLimits{
		Entries:    s.maxIdentityEntries,
		TupleBytes: s.maxIdentityTupleBytes,
	}
}

func (s *session) closeIdentityScopes(depth int) error {
	if !s.hasIdentityConstraints {
		return nil
	}
	return s.doc.identity.CloseScopes(depth, func(err error) error {
		return s.recover(err)
	})
}
