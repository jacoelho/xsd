package xsd

import "strings"

const nilledIdentityKey = "\xff\x1e\x00nil"

func nilledIdentityValue() simpleValue {
	return simpleValue{Identity: nilledIdentityKey}
}

func (s *session) validateSimpleContent(f *frame, line, col int) (bool, error) {
	rt := s.engine.rt
	if f.Nilled {
		return false, nil
	}
	typeID, hasSimpleContent, err := s.simpleContentType(f, line, col)
	if err != nil || !hasSimpleContent {
		return false, err
	}
	rawBytes := s.doc.text[f.TextStart:]
	if len(rt.Identities) == 0 &&
		(f.Element == noElement || (!rt.Elements[f.Element].Fixed.Present && !rt.Elements[f.Element].Default.Present)) {
		ok, rawErr := validateRawSimpleContentFast(rt, typeID, rawBytes)
		if ok {
			if rawErr != nil {
				return false, validation(ErrValidationFacet, line, col, s.pathString(), "invalid simple content: "+rawErr.Error())
			}
			return true, nil
		}
	}
	input := s.simpleContentInput(f, rawBytes)
	if input.prevalidated {
		return s.recordElementSimpleContent(input.value, line, col)
	}
	identityFields, needs := s.simpleContentNeeds(f, typeID)
	value, err := validateSimpleValueMode(rt, typeID, input.text, s.resolveLexicalQNameParts, needs)
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
		if decl.Fixed.Present && value.Canonical != decl.Fixed.Canonical {
			return false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
		}
	}
	if len(identityFields) != 0 {
		if err := s.captureIdentityFields(identityFields, value, line, col); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *session) simpleContentType(f *frame, line, col int) (simpleTypeID, bool, error) {
	if id, ok := f.Type.simple(); ok {
		return id, true, nil
	}
	ct := s.engine.rt.ComplexTypes[f.Type.ID]
	if !ct.simpleContent() {
		return noSimpleType, false, s.validateNonSimpleFixedContent(f, line, col)
	}
	return ct.TextType, true, nil
}

type simpleContentInput struct {
	text         string
	value        simpleValue
	prevalidated bool
}

func (s *session) simpleContentInput(f *frame, rawBytes []byte) simpleContentInput {
	if f.Element != noElement && len(rawBytes) == 0 {
		decl := s.engine.rt.Elements[f.Element]
		if decl.Fixed.Present {
			if decl.Type == f.Type {
				return simpleContentInput{value: decl.Fixed.Value, prevalidated: true}
			}
			return simpleContentInput{text: decl.Fixed.Lexical}
		}
		if decl.Default.Present {
			if decl.Type == f.Type {
				return simpleContentInput{value: decl.Default.Value, prevalidated: true}
			}
			return simpleContentInput{text: decl.Default.Lexical}
		}
	}
	return simpleContentInput{text: s.valueStrings.intern(rawBytes)}
}

func (s *session) simpleContentNeeds(f *frame, typeID simpleTypeID) ([]identityFieldMatch, simpleValueNeed) {
	var identityFields []identityFieldMatch
	if len(s.engine.rt.Identities) != 0 {
		identityFields = s.identityElementFields()
	}
	needIdentity := len(identityFields) != 0
	needCanon := s.needsSimpleContentCanonical(f, typeID, needIdentity)
	var needs simpleValueNeed
	if needCanon {
		needs |= simpleNeedCanonical
	}
	if needIdentity {
		needs |= simpleNeedIdentity
	}
	return identityFields, needs
}

func (s *session) validateNonSimpleFixedContent(f *frame, line, col int) error {
	if f.Element == noElement {
		return nil
	}
	decl := s.engine.rt.Elements[f.Element]
	if !decl.Fixed.Present {
		return nil
	}
	if f.HasChild {
		return validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
	}
	text := s.valueStrings.intern(s.doc.text[f.TextStart:])
	if text != "" && text != decl.Fixed.Lexical {
		return validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
	}
	return nil
}

func (s *session) recordElementSimpleContent(value simpleValue, line, col int) (bool, error) {
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return false, err
	}
	if len(s.engine.rt.Identities) != 0 {
		if err := s.captureIdentityFields(s.identityElementFields(), value, line, col); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *session) needsSimpleContentCanonical(f *frame, typeID simpleTypeID, needIdentity bool) bool {
	if s.simpleIdentityKind(typeID) != simpleIdentityNone {
		return true
	}
	if f.Element != noElement && s.engine.rt.Elements[f.Element].Fixed.Present {
		return true
	}
	return needIdentity
}

func (s *session) identityElementFields() []identityFieldMatch {
	s.doc.identityMatches = s.doc.identityMatches[:0]
	depth := len(s.doc.namePath)
	for i := range s.doc.idSelections {
		sel := &s.doc.idSelections[i]
		ic := &s.engine.rt.Identities[sel.Constraint]
		for _, field := range ic.ElementFields {
			if s.identityFieldPathsMatch(sel.Depth, depth, field.Paths) {
				s.doc.identityMatches = append(s.doc.identityMatches, identityFieldMatch{Selection: i, Field: field.Field})
			}
		}
	}
	return s.doc.identityMatches
}

func (s *session) identityAttributeFields(name qName) []identityFieldMatch {
	s.doc.identityMatches = s.doc.identityMatches[:0]
	depth := len(s.doc.namePath)
	for i := range s.doc.idSelections {
		sel := &s.doc.idSelections[i]
		ic := &s.engine.rt.Identities[sel.Constraint]
		start := len(s.doc.identityMatches)
		for _, field := range ic.AttributeFields[name] {
			if s.identityFieldPathsMatch(sel.Depth, depth, field.Paths) {
				s.doc.identityMatches = append(s.doc.identityMatches, identityFieldMatch{Selection: i, Field: field.Field})
			}
		}
		for _, field := range ic.AttributeWildcardFields {
			if identityMatchExists(s.doc.identityMatches[start:], i, field.Field) {
				continue
			}
			if s.identityFieldAttributePathsMatch(sel.Depth, depth, name, field.Paths) {
				s.doc.identityMatches = append(s.doc.identityMatches, identityFieldMatch{Selection: i, Field: field.Field})
			}
		}
	}
	return s.doc.identityMatches
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
		if s.doc.ids == nil {
			s.doc.ids = make(map[string]string)
		}
		if prev, exists := s.doc.ids[canonical]; exists {
			return validation(ErrValidationType, line, col, path, "duplicate ID "+canonical+" first seen at "+prev)
		}
		if err := s.reserveIdentityEntry(canonical, line, col); err != nil {
			return err
		}
		s.doc.ids[canonical] = path
	}
	for canonical := range xmlFieldsSeq(value.IDRefs) {
		if err := s.reserveIdentityEntry(canonical, line, col); err != nil {
			return err
		}
		s.doc.idrefs = append(s.doc.idrefs, identityRef{Value: canonical, Path: path, Line: line, Col: col})
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
	if len(s.doc.idrefs) == 0 {
		return nil
	}
	for _, ref := range s.doc.idrefs {
		if _, ok := s.doc.ids[ref.Value]; ok {
			continue
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
	if s.maxIdentityScopes > 0 && len(s.doc.idScopes) >= s.maxIdentityScopes {
		return validation(ErrValidationIdentity, line, col, s.pathString(), "identity scope limit exceeded")
	}
	s.doc.idScopes = append(s.doc.idScopes, identityScope{
		Depth:       len(s.doc.namePath),
		Constraints: ids,
	})
	return nil
}

func (s *session) matchIdentitySelectors(line, col int) {
	if len(s.doc.idScopes) == 0 {
		return
	}
	rt := s.engine.rt
	depth := len(s.doc.namePath)
	for scopeIndex := range s.doc.idScopes {
		scope := &s.doc.idScopes[scopeIndex]
		for _, id := range scope.Constraints {
			ic := rt.Identities[id]
			if !s.identitySelectorMatches(scope.Depth, depth, ic.Selector) {
				continue
			}
			fieldStart := len(s.doc.identityFieldValues)
			fieldLen := len(ic.Fields)
			for range fieldLen {
				s.doc.identityFieldValues = append(s.doc.identityFieldValues, identityFieldValue{})
			}
			s.doc.idSelections = append(s.doc.idSelections, identitySelection{
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
	rel := s.doc.namePath[baseDepth:currentDepth]
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
	if !step.Wildcard {
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
		if match.Selection < 0 || match.Selection >= len(s.doc.idSelections) {
			return internalInvariant("identity field match references invalid selection")
		}
		sel := &s.doc.idSelections[match.Selection]
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

func (s *session) captureIdentityXSIAttribute(a *streamAttr, line, col int) error {
	name, ok := s.engine.rt.Names.LookupQName(a.Name.Space, a.Name.Local)
	if !ok {
		return nil
	}
	lexical := a.stringValue(&s.valueStrings)
	value := simpleValue{Canonical: normalizeWhitespace(lexical, whitespaceCollapse), Type: s.engine.rt.Builtin.String}
	switch a.Name.Local {
	case xsiAttrNil:
		v, err := validateSimpleValueMode(s.engine.rt, s.engine.rt.Builtin.Boolean, lexical, nil, simpleNeedCanonical)
		if err != nil {
			return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:nil: "+err.Error())
		}
		value = v
	case xsiAttrType:
		v, err := validateSimpleValueMode(s.engine.rt, s.engine.rt.Builtin.qName, lexical, s.resolveLexicalQNameParts, simpleNeedCanonical)
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
		if match.Selection < 0 || match.Selection >= len(s.doc.idSelections) {
			return internalInvariant("identity field match references invalid selection")
		}
		return validation(ErrValidationIdentity, line, col, s.doc.idSelections[match.Selection].Path, "identity field has no simple value")
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
	if len(s.doc.idSelections) == 0 {
		return nil
	}
	orig := s.doc.idSelections
	dst := s.doc.idSelections[:0]
	for i := range s.doc.idSelections {
		sel := s.doc.idSelections[i]
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
				s.doc.idSelections = dst
				s.truncateIdentityFieldValues()
				return recoverErr
			}
			continue
		}
		clear(s.identitySelectionFields(sel))
	}
	clear(orig[len(dst):])
	s.doc.idSelections = dst
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
	scope := &s.doc.idScopes[sel.Scope]
	switch ic.Kind {
	case identityUnique, identityKey:
		if scope.Tables == nil {
			scope.Tables = make(map[identityConstraintID]map[string]identityTableEntry)
		}
		table := scope.Tables[sel.Constraint]
		if table == nil {
			table = make(map[string]identityTableEntry)
			scope.Tables[sel.Constraint] = table
		}
		if prev, exists := table[key]; exists {
			return validation(ErrValidationIdentity, line, col, sel.Path, "duplicate identity value first seen at "+prev.Path)
		}
		if err := s.reserveIdentityEntry(key, line, col); err != nil {
			return err
		}
		table[key] = identityTableEntry{Path: sel.Path}
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
	return s.doc.identityFieldValues[sel.FieldStart : sel.FieldStart+sel.FieldLen]
}

func (s *session) truncateIdentityFieldValues() {
	n := 0
	for _, sel := range s.doc.idSelections {
		end := sel.FieldStart + sel.FieldLen
		if end > n {
			n = end
		}
	}
	clear(s.doc.identityFieldValues[n:])
	s.doc.identityFieldValues = s.doc.identityFieldValues[:n]
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
	if s.maxIdentityEntries > 0 && s.doc.identityEntries >= s.maxIdentityEntries {
		return validation(ErrValidationIdentity, line, col, s.pathString(), "identity entry limit exceeded")
	}
	s.doc.identityEntries++
	return nil
}

func (s *session) closeIdentityScopes(depth int) error {
	for len(s.doc.idScopes) > 0 && s.doc.idScopes[len(s.doc.idScopes)-1].Depth == depth {
		scope := &s.doc.idScopes[len(s.doc.idScopes)-1]
		for _, ref := range scope.Refs {
			entry, ok := scope.Tables[ref.Refer][ref.Key]
			if !ok || entry.Conflict {
				if err := s.recover(validation(ErrValidationIdentity, ref.Line, ref.Col, ref.Path, "keyref does not resolve")); err != nil {
					return err
				}
			}
		}
		if len(s.doc.idScopes) > 1 {
			s.mergeIdentityTables(&s.doc.idScopes[len(s.doc.idScopes)-2], scope)
		}
		*scope = identityScope{}
		s.doc.idScopes = s.doc.idScopes[:len(s.doc.idScopes)-1]
	}
	return nil
}

func (s *session) mergeIdentityTables(dst, src *identityScope) {
	if len(src.Tables) == 0 {
		return
	}
	if dst.Tables == nil {
		dst.Tables = make(map[identityConstraintID]map[string]identityTableEntry)
	}
	for id, srcTable := range src.Tables {
		dstTable := dst.Tables[id]
		if dstTable == nil {
			// The source scope is discarded right after the merge, so the
			// table can change owner without copying.
			dst.Tables[id] = srcTable
			continue
		}
		for key, entry := range srcTable {
			prev, exists := dstTable[key]
			switch {
			case !exists:
				dstTable[key] = entry
			case prev.Conflict:
			case entry.Conflict || prev.Path != entry.Path:
				dstTable[key] = identityTableEntry{Path: prev.Path, Conflict: true}
			}
		}
	}
}
