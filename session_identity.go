package xsd

import (
	"encoding/xml"
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
			text = decl.Fixed
		} else if decl.HasDefault {
			text = decl.Default
		}
	}
	needCanon := s.needsSimpleContentCanonical(f, typeID)
	canon, err := validateSimpleValueMode(rt, typeID, text, s.resolveLexicalQNameValue, needCanon)
	if err != nil {
		if IsUnsupported(err) {
			return "", noSimpleType, false, err
		}
		return "", noSimpleType, false, validation(ErrValidationFacet, line, col, s.pathString(), "invalid simple content: "+err.Error())
	}
	if err := s.recordIdentity(typeID, canon, line, col); err != nil {
		return "", noSimpleType, false, err
	}
	if f.Element != noElement {
		decl := rt.Elements[f.Element]
		if decl.HasFixed {
			fixed, err := validateSimpleValue(rt, typeID, decl.Fixed, s.resolveLexicalQNameValue)
			if err != nil {
				if IsUnsupported(err) {
					return "", noSimpleType, false, err
				}
				return "", noSimpleType, false, validation(ErrValidationFacet, line, col, s.pathString(), "invalid fixed value")
			}
			if canon != fixed {
				return "", noSimpleType, false, validation(ErrValidationElement, line, col, s.pathString(), "fixed element value mismatch")
			}
		}
	}
	return canon, typeID, true, nil
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

func (s *session) recordAttributeIdentity(typeID simpleTypeID, canonical string, line, col int, seenID *bool) error {
	if s.simpleIdentityKind(typeID) == simpleIdentityID {
		if *seenID {
			return validation(ErrValidationType, line, col, s.pathString(), "multiple ID attributes")
		}
		*seenID = true
	}
	return s.recordIdentity(typeID, canonical, line, col)
}

func (s *session) recordIdentity(typeID simpleTypeID, canonical string, line, col int) error {
	switch s.simpleIdentityKind(typeID) {
	case simpleIdentityID:
		if s.ids == nil {
			s.ids = make(map[string]string)
		}
		if prev, exists := s.ids[canonical]; exists {
			return validation(ErrValidationType, line, col, s.pathString(), "duplicate ID "+canonical+" first seen at "+prev)
		}
		s.ids[canonical] = s.pathString()
		return nil
	case simpleIdentityIDREF:
		s.idrefs = append(s.idrefs, identityRef{Value: canonical, Path: s.pathString(), Line: line, Col: col})
		return nil
	case simpleIdentityIDREFList:
		for item := range strings.FieldsSeq(canonical) {
			s.idrefs = append(s.idrefs, identityRef{Value: item, Path: s.pathString(), Line: line, Col: col})
		}
	}
	return nil
}

func (s *session) simpleIdentityKind(typeID simpleTypeID) simpleIdentityKind {
	rt := s.engine.rt
	if typeID == noSimpleType || int(typeID) >= len(rt.SimpleTypes) {
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
			return validation(ErrValidationType, ref.Line, ref.Col, ref.Path, "IDREF does not resolve: "+ref.Value)
		}
		if _, ok := s.ids[ref.Value]; !ok {
			return validation(ErrValidationType, ref.Line, ref.Col, ref.Path, "IDREF does not resolve: "+ref.Value)
		}
	}
	return nil
}

func (s *session) startIdentityScope(elem elementID) {
	if elem == noElement {
		return
	}
	ids := s.engine.rt.Elements[elem].Identity
	if len(ids) == 0 {
		return
	}
	s.idScopes = append(s.idScopes, identityScope{
		Depth:       len(s.namePath),
		Constraints: ids,
	})
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
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := s.engine.rt.Identities[sel.Constraint]
		for fieldIndex, field := range ic.Fields {
			for _, p := range field.Paths {
				if !s.identityFieldAttributeMatches(p, name) {
					continue
				}
				if !s.identityFieldPathMatches(sel.Depth, len(s.namePath), p) {
					continue
				}
				if sel.Present[fieldIndex] {
					return validation(ErrValidationIdentity, line, col, sel.Path, "identity field selects multiple values")
				}
				sel.Values[fieldIndex] = s.identityValue(typeID, value)
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
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := s.engine.rt.Identities[sel.Constraint]
		for fieldIndex, field := range ic.Fields {
			for _, p := range field.Paths {
				if p.Attr {
					continue
				}
				if !s.identityFieldPathMatches(sel.Depth, len(s.namePath), p) {
					continue
				}
				if sel.Present[fieldIndex] {
					return validation(ErrValidationIdentity, line, col, sel.Path, "identity field selects multiple values")
				}
				sel.Values[fieldIndex] = s.identityValue(typeID, value)
				sel.Present[fieldIndex] = true
				break
			}
		}
	}
	return nil
}

func (s *session) captureIdentityComplexElement(rawText string, line, col int) error {
	for i := range s.idSelections {
		sel := &s.idSelections[i]
		ic := s.engine.rt.Identities[sel.Constraint]
		for fieldIndex, field := range ic.Fields {
			for _, p := range field.Paths {
				if p.Attr {
					continue
				}
				if !s.identityFieldPathMatches(sel.Depth, len(s.namePath), p) {
					continue
				}
				if sel.Present[fieldIndex] {
					return validation(ErrValidationIdentity, line, col, sel.Path, "identity field selects multiple values")
				}
				if strings.TrimSpace(rawText) == "" {
					return validation(ErrValidationIdentity, line, col, sel.Path, "identity field has no simple value")
				}
				sel.Values[fieldIndex] = s.identityValue(s.engine.rt.Builtin.String, normalizeWhitespace(rawText, whitespaceCollapse))
				sel.Present[fieldIndex] = true
				break
			}
		}
	}
	return nil
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
			s.idSelections = append(dst, s.idSelections[i+1:]...)
			return err
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
		table[key] = sel.Path
	case identityKeyRef:
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

func (s *session) closeIdentityScopes(depth int) error {
	for len(s.idScopes) > 0 && s.idScopes[len(s.idScopes)-1].Depth == depth {
		scope := &s.idScopes[len(s.idScopes)-1]
		for _, ref := range scope.Refs {
			table := scope.Tables[ref.Refer]
			if table == nil {
				return validation(ErrValidationIdentity, ref.Line, ref.Col, ref.Path, "keyref does not resolve")
			}
			path, ok := table[ref.Key]
			if !ok || path == identityConflictPath {
				return validation(ErrValidationIdentity, ref.Line, ref.Col, ref.Path, "keyref does not resolve")
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
			dstTable = make(map[string]string, len(srcTable))
			dst.Tables[id] = dstTable
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
