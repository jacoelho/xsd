package xsd

import (
	"encoding/xml"
	"maps"
	"strings"
)

const nilledIdentityKey = "\xff\x1e\x00nil"

func nilledIdentityValue() simpleValue {
	return simpleValue{Identity: nilledIdentityKey}
}

func (s *session) validateSimpleContent(f *frame, line, col int) (bool, error) {
	rt := s.engine.rt
	if f.Nilled {
		return false, nil
	}
	rawBytes := s.text[f.TextStart:]
	rawText := s.valueStrings.intern(rawBytes)
	text := rawText
	var typeID simpleTypeID
	if f.Type.Kind == typeSimple {
		typeID = simpleTypeID(f.Type.ID)
	} else {
		ct := rt.ComplexTypes[f.Type.ID]
		if !ct.SimpleValue {
			if f.Element != noElement && rt.Elements[f.Element].HasFixed {
				if f.HasChild {
					return false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
				}
				fixed := rt.Elements[f.Element].Fixed
				if text != "" && text != fixed {
					return false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
				}
			}
			return false, nil
		}
		typeID = ct.TextType
	}
	if f.Element != noElement && rawText == "" {
		decl := rt.Elements[f.Element]
		if decl.HasFixed {
			if decl.Type == f.Type {
				return s.recordElementSimpleContent(decl.FixedValue, line, col)
			}
			text = decl.Fixed
		} else if decl.HasDefault {
			if decl.Type == f.Type {
				return s.recordElementSimpleContent(decl.DefaultValue, line, col)
			}
			text = decl.Default
		}
	}
	identityFields := s.identityElementFields()
	needIdentity := len(identityFields) != 0
	needCanon := s.needsSimpleContentCanonical(f, typeID, needIdentity)
	var needs simpleValueNeed
	if needCanon {
		needs |= simpleNeedCanonical
	}
	if needIdentity {
		needs |= simpleNeedIdentity
	}
	value, err := validateSimpleValueMode(rt, typeID, text, s.resolveLexicalQNameValue, needs)
	if err != nil {
		if IsUnsupported(err) {
			return false, err
		}
		return false, validation(ErrValidationFacet, line, col, s.pathString(), "invalid simple content: "+err.Error())
	}
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return false, err
	}
	if f.Element != noElement {
		decl := rt.Elements[f.Element]
		if decl.HasFixed && value.Canonical != decl.FixedCanonical {
			return false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
		}
	}
	if err := s.captureIdentityFields(identityFields, value, line, col); err != nil {
		return false, err
	}
	return true, nil
}

func (s *session) recordElementSimpleContent(value simpleValue, line, col int) (bool, error) {
	identityFields := s.identityElementFields()
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return false, err
	}
	if err := s.captureIdentityFields(identityFields, value, line, col); err != nil {
		return false, err
	}
	return true, nil
}

func (s *session) needsSimpleContentCanonical(f *frame, typeID simpleTypeID, needIdentity bool) bool {
	if s.simpleIdentityKind(typeID) != simpleIdentityNone {
		return true
	}
	if f.Element != noElement && s.engine.rt.Elements[f.Element].HasFixed {
		return true
	}
	return needIdentity
}

func (s *session) identityElementFields() []identityFieldMatch {
	s.identityMatches = s.identityMatches[:0]
	depth := len(s.namePath)
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := &s.engine.rt.Identities[sel.Constraint]
		for _, field := range ic.ElementFields {
			if s.identityFieldPathsMatch(sel.Depth, depth, field.Paths) {
				s.identityMatches = append(s.identityMatches, identityFieldMatch{Selection: i, Field: field.Field})
			}
		}
	}
	return s.identityMatches
}

func (s *session) identityAttributeFields(name qName) []identityFieldMatch {
	s.identityMatches = s.identityMatches[:0]
	depth := len(s.namePath)
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := &s.engine.rt.Identities[sel.Constraint]
		start := len(s.identityMatches)
		for _, field := range ic.AttributeFields[name] {
			if s.identityFieldPathsMatch(sel.Depth, depth, field.Paths) {
				s.identityMatches = append(s.identityMatches, identityFieldMatch{Selection: i, Field: field.Field})
			}
		}
		for _, field := range ic.AttributeWildcardFields {
			if identityMatchExists(s.identityMatches[start:], i, field.Field) {
				continue
			}
			if s.identityFieldAttributePathsMatch(sel.Depth, depth, name, field.Paths) {
				s.identityMatches = append(s.identityMatches, identityFieldMatch{Selection: i, Field: field.Field})
			}
		}
	}
	return s.identityMatches
}

func (s *session) recordAttributeIdentity(value simpleValue, line, col int, seenID *bool) error {
	if value.IDs != "" {
		if *seenID {
			return validation(ErrValidationType, line, col, s.pathString(), "multiple ID attributes")
		}
		*seenID = true
	}
	return s.recordIdentityValue(value, line, col)
}

func (s *session) recordIdentityValue(value simpleValue, line, col int) error {
	if value.IDs == "" && value.IDRefs == "" {
		return nil
	}
	path := s.pathString()
	for canonical := range xmlFieldsSeq(value.IDs) {
		if s.ids == nil {
			s.ids = make(map[string]string)
		}
		if prev, exists := s.ids[canonical]; exists {
			return validation(ErrValidationType, line, col, path, "duplicate ID "+canonical+" first seen at "+prev)
		}
		if err := s.reserveIdentityEntry(canonical, line, col); err != nil {
			return err
		}
		s.ids[canonical] = path
	}
	for canonical := range xmlFieldsSeq(value.IDRefs) {
		if err := s.reserveIdentityEntry(canonical, line, col); err != nil {
			return err
		}
		s.idrefs = append(s.idrefs, identityRef{Value: canonical, Path: path, Line: line, Col: col})
	}
	return nil
}

func (s *session) simpleIdentityKind(typeID simpleTypeID) simpleIdentityKind {
	rt := s.engine.rt
	if typeID == noSimpleType || !validUint32Index(uint32(typeID), len(rt.SimpleTypes)) {
		return simpleIdentityNone
	}
	return rt.SimpleTypes[typeID].Identity
}

func (s *session) checkIDRefs() error {
	if len(s.idrefs) == 0 {
		return nil
	}
	for _, ref := range s.idrefs {
		if s.ids != nil {
			if _, ok := s.ids[ref.Value]; ok {
				continue
			}
		}
		if err := s.recover(validation(ErrValidationType, ref.Line, ref.Col, ref.Path, "IDREF does not resolve: "+ref.Value)); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) startIdentityScope(elem elementID, line, col int) error {
	if elem == noElement {
		return nil
	}
	ids := s.engine.rt.Elements[elem].Identity
	if len(ids) == 0 {
		return nil
	}
	if s.maxIdentityScopes > 0 && len(s.idScopes) >= s.maxIdentityScopes {
		return validation(ErrValidationIdentity, line, col, s.pathString(), "identity scope limit exceeded")
	}
	s.idScopes = append(s.idScopes, identityScope{
		Depth:       len(s.namePath),
		Constraints: ids,
	})
	return nil
}

func (s *session) matchIdentitySelectors(line, col int) {
	rt := s.engine.rt
	depth := len(s.namePath)
	for scopeIndex := range s.idScopes {
		scope := &s.idScopes[scopeIndex]
		for _, id := range scope.Constraints {
			ic := rt.Identities[id]
			if !s.identitySelectorMatches(scope.Depth, depth, ic.Selector) {
				continue
			}
			fieldStart := len(s.identityFieldValues)
			fieldLen := len(ic.Fields)
			for range fieldLen {
				s.identityFieldValues = append(s.identityFieldValues, identityFieldValue{})
			}
			s.idSelections = append(s.idSelections, identitySelection{
				Scope:      scopeIndex,
				Constraint: id,
				Depth:      depth,
				FieldStart: fieldStart,
				FieldLen:   fieldLen,
				Path:       s.pathString(),
				Line:       line,
				Col:        col,
			})
		}
	}
}

func (s *session) identitySelectorMatches(scopeDepth, currentDepth int, paths []identityPath) bool {
	for _, p := range paths {
		pattern := identityPathPattern{descendant: p.Descendant, self: p.Self, steps: p.Steps}
		if s.identityPathMatches(scopeDepth, currentDepth, pattern) {
			return true
		}
	}
	return false
}

type identityPathPattern struct {
	steps      []identityStep
	descendant bool
	self       bool
}

func (s *session) identityPathMatches(baseDepth, currentDepth int, p identityPathPattern) bool {
	if p.self {
		return currentDepth == baseDepth
	}
	if currentDepth < baseDepth {
		return false
	}
	rel := s.namePath[baseDepth:currentDepth]
	if p.descendant {
		if len(rel) < len(p.steps) {
			return false
		}
		rel = rel[len(rel)-len(p.steps):]
	} else if len(rel) != len(p.steps) {
		return false
	}
	for i := range p.steps {
		if !s.identityStepMatches(rel[i], p.steps[i]) {
			return false
		}
	}
	return true
}

func (s *session) identityStepMatches(rn runtimeName, step identityStep) bool {
	if !step.wildcard {
		return rn.Known && rn.Name == step.Name
	}
	if !step.NamespaceSet {
		return true
	}
	if rn.Known {
		return rn.Name.Namespace == step.Namespace
	}
	return rn.NS == s.engine.rt.Names.Namespace(step.Namespace)
}

func (s *session) captureIdentityFields(fields []identityFieldMatch, value simpleValue, line, col int) error {
	if len(fields) == 0 {
		return nil
	}
	var identity string
	identitySet := false
	for _, match := range fields {
		if match.Selection < 0 || match.Selection >= len(s.idSelections) {
			return internalInvariant("identity field match references invalid selection")
		}
		sel := &s.idSelections[match.Selection]
		if match.Field < 0 || match.Field >= sel.FieldLen {
			return internalInvariant("identity field match references invalid field")
		}
		selectionFields := s.identitySelectionFields(*sel)
		field := &selectionFields[match.Field]
		if field.Present {
			return validation(ErrValidationIdentity, line, col, sel.Path, "identity field selects multiple values")
		}
		if !identitySet {
			identity = s.identityValue(value)
			identitySet = true
		}
		field.Value = identity
		field.Present = true
	}
	return nil
}

func (s *session) captureIdentityXSIAttribute(a xml.Attr, line, col int) error {
	name, ok := s.engine.rt.Names.LookupQName(a.Name.Space, a.Name.Local)
	if !ok {
		return nil
	}
	value := simpleValue{Canonical: normalizeWhitespace(a.Value, whitespaceCollapse), Type: s.engine.rt.Builtin.String}
	switch a.Name.Local {
	case xsiAttrNil:
		v, err := validateSimpleValueMode(s.engine.rt, s.engine.rt.Builtin.Boolean, a.Value, nil, simpleNeedCanonical)
		if err != nil {
			return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:nil: "+err.Error())
		}
		value = v
	case xsiAttrType:
		v, err := validateSimpleValueMode(s.engine.rt, s.engine.rt.Builtin.qName, a.Value, s.resolveLexicalQNameValue, simpleNeedCanonical)
		if err != nil {
			return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:type: "+err.Error())
		}
		value = v
	}
	return s.captureIdentityFields(s.identityAttributeFields(name), value, line, col)
}

func (s *session) identityFieldPathsMatch(selectedDepth, currentDepth int, paths []identityFieldPath) bool {
	for _, path := range paths {
		if s.identityFieldPathMatches(selectedDepth, currentDepth, path) {
			return true
		}
	}
	return false
}

func (s *session) identityFieldAttributePathsMatch(selectedDepth, currentDepth int, name qName, paths []identityFieldPath) bool {
	for _, path := range paths {
		if s.identityFieldAttributeMatches(path, name) && s.identityFieldPathMatches(selectedDepth, currentDepth, path) {
			return true
		}
	}
	return false
}

func identityMatchExists(matches []identityFieldMatch, selection, field int) bool {
	for _, match := range matches {
		if match.Selection == selection && match.Field == field {
			return true
		}
	}
	return false
}

func (s *session) identityFieldAttributeMatches(p identityFieldPath, name qName) bool {
	if !p.Attr {
		return false
	}
	if !p.AttrWildcard {
		return p.Attribute == name
	}
	return !p.AttrNamespaceSet || p.AttrNamespace == name.Namespace
}

func (s *session) captureIdentityComplexElement(rawText []byte, line, col int) error {
	fields := s.identityElementFields()
	if len(fields) == 0 {
		return nil
	}
	if isXMLWhitespaceBytes(rawText) {
		match := fields[0]
		if match.Selection < 0 || match.Selection >= len(s.idSelections) {
			return internalInvariant("identity field match references invalid selection")
		}
		return validation(ErrValidationIdentity, line, col, s.idSelections[match.Selection].Path, "identity field has no simple value")
	}
	text := string(rawText)
	return s.captureIdentityFields(fields, simpleValue{
		Canonical: normalizeWhitespace(text, whitespaceCollapse),
		Type:      s.engine.rt.Builtin.String,
	}, line, col)
}

func (s *session) identityValue(value simpleValue) string {
	if value.Identity != "" {
		return value.Identity
	}
	if value.Type == noSimpleType {
		return "\xff\x1e" + value.Canonical
	}
	typ := s.engine.rt.SimpleTypes[value.Type]
	primitive := typ.Primitive
	return simpleIdentityKey(primitive, value.Canonical)
}

func (s *session) identityFieldPathMatches(selectedDepth, currentDepth int, p identityFieldPath) bool {
	if p.Self {
		return currentDepth == selectedDepth
	}
	pattern := identityPathPattern{descendant: p.Descendant, steps: p.Steps}
	return s.identityPathMatches(selectedDepth, currentDepth, pattern)
}

func (s *session) finishIdentitySelections(depth, line, col int) error {
	orig := s.idSelections
	dst := s.idSelections[:0]
	for i := range s.idSelections {
		sel := s.idSelections[i]
		if sel.Depth != depth {
			dst = append(dst, sel)
			continue
		}
		if err := s.finishIdentitySelection(sel, line, col); err != nil {
			clear(s.identitySelectionFields(sel))
			recoverErr := s.recover(err)
			if recoverErr != nil {
				dst = append(dst, orig[i+1:]...)
				clear(orig[len(dst):])
				s.idSelections = dst
				s.truncateIdentityFieldValues()
				return recoverErr
			}
			continue
		}
		clear(s.identitySelectionFields(sel))
	}
	clear(orig[len(dst):])
	s.idSelections = dst
	s.truncateIdentityFieldValues()
	return nil
}

func (s *session) finishIdentitySelection(sel identitySelection, line, col int) error {
	rt := s.engine.rt
	ic := rt.Identities[sel.Constraint]
	fields := s.identitySelectionFields(sel)
	for _, field := range fields {
		if !field.Present {
			if ic.Kind == identityKey {
				return validation(ErrValidationIdentity, line, col, sel.Path, "key field is missing")
			}
			return nil
		}
	}
	key, err := s.identityTupleKey(fields, line, col)
	if err != nil {
		return err
	}
	scope := &s.idScopes[sel.Scope]
	switch ic.Kind {
	case identityUnique, identityKey:
		if scope.Tables == nil {
			scope.Tables = make(map[identityConstraintID]map[string]string)
		}
		table := scope.Tables[sel.Constraint]
		if table == nil {
			table = make(map[string]string)
			scope.Tables[sel.Constraint] = table
		}
		if prev, exists := table[key]; exists {
			return validation(ErrValidationIdentity, line, col, sel.Path, "duplicate identity value first seen at "+prev)
		}
		if err := s.reserveIdentityEntry(key, line, col); err != nil {
			return err
		}
		table[key] = sel.Path
	case identityKeyRef:
		if err := s.reserveIdentityEntry(key, line, col); err != nil {
			return err
		}
		scope.Refs = append(scope.Refs, identityTupleRef{
			Constraint: sel.Constraint,
			Refer:      ic.Refer,
			Key:        key,
			Path:       sel.Path,
			Line:       sel.Line,
			Col:        sel.Col,
		})
	}
	return nil
}

func (s *session) identitySelectionFields(sel identitySelection) []identityFieldValue {
	return s.identityFieldValues[sel.FieldStart : sel.FieldStart+sel.FieldLen]
}

func (s *session) truncateIdentityFieldValues() {
	n := 0
	for _, sel := range s.idSelections {
		end := sel.FieldStart + sel.FieldLen
		if end > n {
			n = end
		}
	}
	clear(s.identityFieldValues[n:])
	s.identityFieldValues = s.identityFieldValues[:n]
}

func (s *session) identityTupleKey(fields []identityFieldValue, line, col int) (string, error) {
	size := int64(0)
	for i, field := range fields {
		if i > 0 {
			size++
		}
		size += int64(len(field.Value))
		if s.maxIdentityTupleBytes > 0 && size > s.maxIdentityTupleBytes {
			return "", validation(ErrValidationIdentity, line, col, s.pathString(), "identity tuple byte limit exceeded")
		}
	}
	if len(fields) == 1 {
		return fields[0].Value, nil
	}
	var b strings.Builder
	b.Grow(int(size))
	for i, field := range fields {
		if i > 0 {
			b.WriteByte('\x1f')
		}
		b.WriteString(field.Value)
	}
	return b.String(), nil
}

func (s *session) reserveIdentityEntry(key string, line, col int) error {
	if s.maxIdentityTupleBytes > 0 && int64(len(key)) > s.maxIdentityTupleBytes {
		return validation(ErrValidationIdentity, line, col, s.pathString(), "identity tuple byte limit exceeded")
	}
	if s.maxIdentityEntries > 0 && s.identityEntries >= s.maxIdentityEntries {
		return validation(ErrValidationIdentity, line, col, s.pathString(), "identity entry limit exceeded")
	}
	s.identityEntries++
	return nil
}

func (s *session) closeIdentityScopes(depth int) error {
	for len(s.idScopes) > 0 && s.idScopes[len(s.idScopes)-1].Depth == depth {
		scope := &s.idScopes[len(s.idScopes)-1]
		for _, ref := range scope.Refs {
			table := scope.Tables[ref.Refer]
			if table == nil {
				recoverErr := s.recover(validation(ErrValidationIdentity, ref.Line, ref.Col, ref.Path, "keyref does not resolve"))
				if recoverErr != nil {
					return recoverErr
				}
				continue
			}
			path, ok := table[ref.Key]
			if !ok || path == identityConflictPath {
				if err := s.recover(validation(ErrValidationIdentity, ref.Line, ref.Col, ref.Path, "keyref does not resolve")); err != nil {
					return err
				}
			}
		}
		if len(s.idScopes) > 1 {
			s.mergeIdentityTables(&s.idScopes[len(s.idScopes)-2], scope)
		}
		*scope = identityScope{}
		s.idScopes = s.idScopes[:len(s.idScopes)-1]
	}
	return nil
}

func (s *session) mergeIdentityTables(dst, src *identityScope) {
	if len(src.Tables) == 0 {
		return
	}
	if dst.Tables == nil {
		dst.Tables = make(map[identityConstraintID]map[string]string)
	}
	for id, srcTable := range src.Tables {
		dstTable := dst.Tables[id]
		if dstTable == nil {
			dst.Tables[id] = maps.Clone(srcTable)
			continue
		}
		for key, path := range srcTable {
			prev, exists := dstTable[key]
			switch {
			case !exists:
				dstTable[key] = path
			case prev != path:
				dstTable[key] = identityConflictPath
			}
		}
	}
}
