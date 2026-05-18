package xsd

import (
	"encoding/xml"
	"maps"
	"strings"
)

func (s *session) validateSimpleContent(f *frame, line, col int) (string, simpleTypeID, bool, error) {
	rt := s.engine.rt
	if f.Nilled {
		return "", noSimpleType, false, nil
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
					return "", noSimpleType, false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
				}
				fixed := rt.Elements[f.Element].Fixed
				if text != "" && text != fixed {
					return "", noSimpleType, false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
				}
			}
			return "", noSimpleType, false, nil
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
	needCanon := s.needsSimpleContentCanonical(f, typeID)
	value, err := validateSimpleValueMode(rt, typeID, text, s.resolveLexicalQNameValue, needCanon)
	if err != nil {
		if IsUnsupported(err) {
			return "", noSimpleType, false, err
		}
		return "", noSimpleType, false, validation(ErrValidationFacet, line, col, s.pathString(), "invalid simple content: "+err.Error())
	}
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return "", noSimpleType, false, err
	}
	if f.Element != noElement {
		decl := rt.Elements[f.Element]
		if decl.HasFixed && value.Canonical != decl.FixedCanonical {
			return "", noSimpleType, false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
		}
	}
	return value.Canonical, value.Type, true, nil
}

func (s *session) recordElementSimpleContent(value simpleValue, line, col int) (string, simpleTypeID, bool, error) {
	if err := s.recordIdentityValue(value, line, col); err != nil {
		return "", noSimpleType, false, err
	}
	return value.Canonical, value.Type, true, nil
}

func (s *session) needsSimpleContentCanonical(f *frame, typeID simpleTypeID) bool {
	if s.simpleIdentityKind(typeID) != simpleIdentityNone {
		return true
	}
	if f.Element != noElement && s.engine.rt.Elements[f.Element].HasFixed {
		return true
	}
	return s.needsIdentityElementValue()
}

func (s *session) needsIdentityElementValue() bool {
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := s.engine.rt.Identities[sel.Constraint]
		for _, field := range ic.Fields {
			for _, p := range field.Paths {
				if p.Attr {
					continue
				}
				if s.identityFieldPathMatches(sel.Depth, len(s.namePath), p) {
					return true
				}
			}
		}
	}
	return false
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
	for canonical := range strings.FieldsSeq(value.IDs) {
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
	for canonical := range strings.FieldsSeq(value.IDRefs) {
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
		if s.ids == nil {
			if err := s.recover(validation(ErrValidationType, ref.Line, ref.Col, ref.Path, "IDREF does not resolve: "+ref.Value)); err != nil {
				return err
			}
			continue
		}
		if _, ok := s.ids[ref.Value]; !ok {
			if err := s.recover(validation(ErrValidationType, ref.Line, ref.Col, ref.Path, "IDREF does not resolve: "+ref.Value)); err != nil {
				return err
			}
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
			s.idSelections = append(s.idSelections, identitySelection{
				Scope:      scopeIndex,
				Constraint: id,
				Depth:      depth,
				Values:     make([]string, len(ic.Fields)),
				Present:    make([]bool, len(ic.Fields)),
				Path:       s.pathString(),
				Line:       line,
				Col:        col,
			})
		}
	}
}

func (s *session) identitySelectorMatches(scopeDepth, currentDepth int, paths []identityPath) bool {
	for _, p := range paths {
		if s.identityPathMatches(scopeDepth, currentDepth, p.Descendant, p.Self, p.Steps) {
			return true
		}
	}
	return false
}

func (s *session) identityPathMatches(baseDepth, currentDepth int, descendant, self bool, steps []identityStep) bool {
	if self {
		return currentDepth == baseDepth
	}
	if currentDepth < baseDepth {
		return false
	}
	rel := s.namePath[baseDepth:currentDepth]
	if descendant {
		if len(rel) < len(steps) {
			return false
		}
		rel = rel[len(rel)-len(steps):]
	} else if len(rel) != len(steps) {
		return false
	}
	for i := range steps {
		if !s.identityStepMatches(rel[i], steps[i]) {
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

func (s *session) captureIdentityAttribute(name qName, typeID simpleTypeID, value string, line, col int) error {
	return s.captureIdentityFields(
		func(p identityFieldPath) bool {
			return s.identityFieldAttributeMatches(p, name)
		},
		func(string) (simpleTypeID, string, error) {
			return typeID, value, nil
		},
		line,
		col,
	)
}

func (s *session) captureIdentityFields(match func(identityFieldPath) bool, value func(string) (simpleTypeID, string, error), line, col int) error {
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := s.engine.rt.Identities[sel.Constraint]
		for fieldIndex, field := range ic.Fields {
			for _, p := range field.Paths {
				if !match(p) || !s.identityFieldPathMatches(sel.Depth, len(s.namePath), p) {
					continue
				}
				if sel.Present[fieldIndex] {
					return validation(ErrValidationIdentity, line, col, sel.Path, "identity field selects multiple values")
				}
				typeID, canonical, err := value(sel.Path)
				if err != nil {
					return err
				}
				sel.Values[fieldIndex] = s.identityValue(typeID, canonical)
				sel.Present[fieldIndex] = true
				break
			}
		}
	}
	return nil
}

func (s *session) captureIdentityXSIAttribute(a xml.Attr, line, col int) error {
	name, ok := s.engine.rt.Names.LookupQName(a.Name.Space, a.Name.Local)
	if !ok {
		return nil
	}
	typeID := s.engine.rt.Builtin.String
	value := normalizeWhitespace(a.Value, whitespaceCollapse)
	switch a.Name.Local {
	case "nil":
		typeID = s.engine.rt.Builtin.Boolean
		canon, err := validateSimpleValue(s.engine.rt, typeID, a.Value, nil)
		if err != nil {
			return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:nil: "+err.Error())
		}
		value = canon
	case "type":
		typeID = s.engine.rt.Builtin.qName
		canon, err := validateSimpleValue(s.engine.rt, typeID, a.Value, s.resolveLexicalQNameValue)
		if err != nil {
			return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:type: "+err.Error())
		}
		value = canon
	}
	return s.captureIdentityAttribute(name, typeID, value, line, col)
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

func (s *session) captureIdentityElement(typeID simpleTypeID, value string, line, col int) error {
	return s.captureIdentityFields(
		func(p identityFieldPath) bool {
			return !p.Attr
		},
		func(string) (simpleTypeID, string, error) {
			return typeID, value, nil
		},
		line,
		col,
	)
}

func (s *session) captureIdentityComplexElement(rawText string, line, col int) error {
	return s.captureIdentityFields(
		func(p identityFieldPath) bool {
			return !p.Attr
		},
		func(path string) (simpleTypeID, string, error) {
			if strings.TrimSpace(rawText) == "" {
				return noSimpleType, "", validation(ErrValidationIdentity, line, col, path, "identity field has no simple value")
			}
			return s.engine.rt.Builtin.String, normalizeWhitespace(rawText, whitespaceCollapse), nil
		},
		line,
		col,
	)
}

func (s *session) identityValue(typeID simpleTypeID, canonical string) string {
	if typeID == noSimpleType {
		return "\xff\x1e" + canonical
	}
	primitive := s.engine.rt.SimpleTypes[typeID].Primitive
	return string([]byte{byte(primitive)}) + "\x1e" + canonical
}

func (s *session) identityFieldPathMatches(selectedDepth, currentDepth int, p identityFieldPath) bool {
	if p.Self {
		return currentDepth == selectedDepth
	}
	return s.identityPathMatches(selectedDepth, currentDepth, p.Descendant, false, p.Steps)
}

func (s *session) finishIdentitySelections(depth, line, col int) error {
	dst := s.idSelections[:0]
	for i := range s.idSelections {
		sel := s.idSelections[i]
		if sel.Depth != depth {
			dst = append(dst, sel)
			continue
		}
		if err := s.finishIdentitySelection(sel, line, col); err != nil {
			recoverErr := s.recover(err)
			if recoverErr != nil {
				s.idSelections = append(dst, s.idSelections[i+1:]...)
				return recoverErr
			}
		}
	}
	s.idSelections = dst
	return nil
}

func (s *session) finishIdentitySelection(sel identitySelection, line, col int) error {
	rt := s.engine.rt
	ic := rt.Identities[sel.Constraint]
	complete := true
	for _, ok := range sel.Present {
		complete = complete && ok
	}
	if !complete {
		if ic.Kind == identityKey {
			return validation(ErrValidationIdentity, line, col, sel.Path, "key field is missing")
		}
		return nil
	}
	if err := s.checkIdentityTupleBytes(sel.Values, line, col); err != nil {
		return err
	}
	key := strings.Join(sel.Values, "\x1f")
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

func (s *session) checkIdentityTupleBytes(values []string, line, col int) error {
	if s.maxIdentityTupleBytes <= 0 {
		return nil
	}
	size := int64(0)
	for i, v := range values {
		if i > 0 {
			size++
		}
		size += int64(len(v))
		if size > s.maxIdentityTupleBytes {
			return validation(ErrValidationIdentity, line, col, s.pathString(), "identity tuple byte limit exceeded")
		}
	}
	return nil
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
