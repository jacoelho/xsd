package xsd

import (
	"encoding/xml"
	"fmt"
)

type matchResult struct {
	element       elementID
	skip          bool
	strictMissing bool
}

func noMatch() matchResult {
	return matchResult{element: noElement}
}

func (m matchResult) child(rt *runtimeSchema) acceptedChild {
	if m.element == noElement {
		return acceptedChild{element: noElement, typ: anyType(rt), skip: m.skip}
	}
	decl := rt.Elements[m.element]
	return acceptedChild{element: m.element, typ: decl.Type, skip: m.skip}
}

func anyTypeChild(rt *runtimeSchema, skip bool) acceptedChild {
	return acceptedChild{element: noElement, typ: anyType(rt), skip: skip}
}

type acceptedChild struct {
	element elementID
	typ     typeID
	skip    bool
}

func (s *session) acceptChild(parent *frame, rn runtimeName, attrs []xml.Attr, line, col int) (acceptedChild, error) {
	rt := s.engine.rt
	if parent.Skip {
		return anyTypeChild(rt, true), nil
	}
	if parent.Nilled {
		return acceptedChild{}, validation(ErrValidationNil, line, col, s.pathString(), "nilled element must be empty")
	}
	if parent.Type.Kind == typeSimple {
		return acceptedChild{}, validation(ErrValidationContent, line, col, s.pathString(), "simple type cannot contain child elements")
	}
	ct := rt.ComplexTypes[parent.Type.ID]
	if ct.SimpleValue {
		return acceptedChild{}, validation(ErrValidationContent, line, col, s.pathString(), "simple content cannot contain child elements")
	}
	if parent.Model == noContentModel {
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "unexpected child element "+rn.Local)
	}
	model := rt.CompiledModels[parent.Model]
	var match matchResult
	var ok bool
	var err error
	switch model.Kind {
	case compiledModelAny:
		return s.acceptAny(rn, wildcard{Mode: wildAny, Process: processLax}, line, col)
	case compiledModelAll:
		match, ok, err = s.acceptAllChild(parent, model, rn, attrs)
	case compiledModelDFA:
		match, ok, err = s.acceptDFAChild(parent, model, rn, attrs)
	default:
		ok = false
	}
	if err != nil {
		return acceptedChild{}, err
	}
	if !ok {
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "unexpected child element "+rn.Local)
	}
	if match.strictMissing {
		if s.hasSchemaLocationHint(rn.NS) {
			return acceptedChild{}, s.unsupportedSchemaLocation(line, col, "element", rn)
		}
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "wildcard requires declared element "+rn.Local)
	}
	return match.child(rt), nil
}

func (s *session) acceptAny(rn runtimeName, w wildcard, line, col int) (acceptedChild, error) {
	rt := s.engine.rt
	if !wildcardMatches(rt, w, rn) {
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "element is not allowed by wildcard "+rn.Local)
	}
	if rn.Known {
		if id, ok := rt.GlobalElements[rn.Name]; ok && w.Process != processSkip {
			return acceptedChild{element: id, typ: rt.Elements[id].Type}, nil
		}
	}
	if w.Process == processSkip {
		return anyTypeChild(rt, true), nil
	}
	if w.Process == processStrict {
		if s.hasSchemaLocationHint(rn.NS) {
			return acceptedChild{}, s.unsupportedSchemaLocation(line, col, "element", rn)
		}
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "wildcard requires declared element "+rn.Local)
	}
	return anyTypeChild(rt, false), nil
}

func (s *session) acceptAllChild(f *frame, model compiledModel, rn runtimeName, attrs []xml.Attr) (matchResult, bool, error) {
	for i, term := range model.All {
		seen, err := s.allSeen(f, i)
		if err != nil {
			return noMatch(), false, err
		}
		if seen {
			continue
		}
		match, ok := s.matchDirectParticle(term.Particle, rn, attrs)
		if !ok {
			continue
		}
		if err := s.setAllSeen(f, i); err != nil {
			return noMatch(), false, err
		}
		return match, true, nil
	}
	return noMatch(), false, nil
}

func (s *session) acceptDFAChild(f *frame, model compiledModel, rn runtimeName, attrs []xml.Attr) (matchResult, bool, error) {
	row, err := s.dfaRow(model, f.State)
	if err != nil {
		return noMatch(), false, err
	}
	for _, edge := range row.Edges {
		match, matched := s.matchDirectParticle(edge.Particle, rn, attrs)
		if !matched {
			continue
		}
		if ok, err := s.advanceDFA(f, model, edge); err != nil {
			return noMatch(), false, err
		} else if !ok {
			continue
		}
		return match, true, nil
	}
	return noMatch(), false, nil
}

func (s *session) matchDirectParticle(p particle, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	rt := s.engine.rt
	switch p.Kind {
	case particleElement:
		if rn.Known && rt.Elements[p.Element].Name == rn.Name {
			return matchResult{element: p.Element}, true
		}
		if rn.Known {
			for _, member := range rt.Substitutions[p.Element] {
				if rt.Elements[member].Name == rn.Name && s.substitutionAllowed(p.Element, member) {
					return matchResult{element: member}, true
				}
			}
		}
	case particleWildcard:
		w := rt.Wildcards[p.wildcard]
		if wildcardMatches(rt, w, rn) {
			if w.Process == processStrict {
				if rn.Known {
					if id, ok := rt.GlobalElements[rn.Name]; ok {
						return matchResult{element: id}, true
					}
				}
				if hasXSIType(attrs) {
					return noMatch(), true
				}
				return matchResult{strictMissing: true}, true
			}
			if w.Process == processSkip {
				return matchResult{element: noElement, skip: true}, true
			}
			if rn.Known && w.Process == processLax {
				if id, ok := rt.GlobalElements[rn.Name]; ok {
					return matchResult{element: id}, true
				}
			}
			return noMatch(), true
		}
	}
	return noMatch(), false
}

func (s *session) substitutionAllowed(headID, memberID elementID) bool {
	rt := s.engine.rt
	head := rt.Elements[headID]
	member := rt.Elements[memberID]
	if head.Block&blockSubstitution != 0 {
		return false
	}
	return rt.substitutionDerivationAllowed(member.Type, head.Type, head.Block)
}

func hasXSIType(attrs []xml.Attr) bool {
	for _, a := range attrs {
		if a.Name.Space == xsiNamespaceURI && a.Name.Local == "type" {
			return true
		}
	}
	return false
}

func (s *session) end(line, col int, ee xml.EndElement) error {
	if len(s.stack) == 0 {
		return validation(ErrValidationXML, line, col, s.pathString(), "unexpected end element")
	}
	translated, err := s.translateName(ee.Name, true, line, col)
	if err != nil {
		return err
	}
	ee.Name = translated
	expected := s.elementNames[len(s.elementNames)-1]
	if ee.Name != expected {
		return validation(ErrValidationXML, line, col, s.pathString(), "end element </"+formatXMLName(ee.Name)+"> does not match start element <"+formatXMLName(expected)+">")
	}
	f := &s.stack[len(s.stack)-1]
	stop := s.validateFrameEnd(f, line, col)
	if stop == nil {
		stop = s.finishFrameIdentity(line, col)
	}
	s.allBits = s.allBits[:f.BitBase]
	s.text = s.text[:f.TextStart]
	s.stack = s.stack[:len(s.stack)-1]
	if len(s.path) > 0 {
		s.path = s.path[:len(s.path)-1]
	}
	if len(s.namePath) > 0 {
		s.namePath = s.namePath[:len(s.namePath)-1]
	}
	if len(s.elementNames) > 0 {
		s.elementNames = s.elementNames[:len(s.elementNames)-1]
	}
	s.ns.pop()
	return stop
}

func (s *session) validateFrameEnd(f *frame, line, col int) error {
	if f.Skip {
		return nil
	}
	if f.Nilled && (f.HasChild || f.HasText) {
		err := validation(ErrValidationNil, line, col, s.pathString(), "nilled element must be empty")
		if recoverErr := s.recover(err); recoverErr != nil {
			return recoverErr
		}
	}
	if !f.Nilled {
		if err := s.completeFrame(f, line, col); err != nil {
			if recoverErr := s.recover(err); recoverErr != nil {
				return recoverErr
			}
		}
	}
	content, err := s.validateSimpleContent(f, line, col)
	if err != nil {
		return s.recover(err)
	}
	return s.captureEndIdentity(f, content, line, col)
}

func (s *session) captureEndIdentity(f *frame, content simpleContentValue, line, col int) error {
	var err error
	switch {
	case content.Captured:
		err = s.captureIdentityElement(content.Value, line, col)
	case f.Nilled && f.Element != noElement:
		err = s.captureIdentityElement(nilledIdentityValue(), line, col)
	case f.Type.Kind == typeComplex && !s.engine.rt.ComplexTypes[f.Type.ID].SimpleValue:
		err = s.captureIdentityComplexElement(string(s.text[f.TextStart:]), line, col)
	}
	return s.recover(err)
}

func (s *session) finishFrameIdentity(line, col int) error {
	if err := s.finishIdentitySelections(len(s.namePath), line, col); err != nil {
		return err
	}
	return s.closeIdentityScopes(len(s.namePath))
}

func (s *session) completeFrame(f *frame, line, col int) error {
	if f.Type.Kind != typeComplex || f.Model == noContentModel {
		return nil
	}
	model := s.engine.rt.CompiledModels[f.Model]
	var err error
	switch model.Kind {
	case compiledModelEmpty, compiledModelAny:
		err = nil
	case compiledModelAll:
		err = s.completeAllModel(f, model, line, col)
	case compiledModelDFA:
		err = s.completeDFAModel(f, model, line, col)
	default:
		err = &Error{Category: InternalErrorCategory, Code: ErrInternalInvariant, Line: line, Column: col, Path: s.pathString(), Message: "compiled content model has invalid kind"}
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *session) completeAllModel(f *frame, model compiledModel, line, col int) error {
	empty := true
	var missingRequired bool
	for i, term := range model.All {
		seen, err := s.allSeen(f, i)
		if err != nil {
			return err
		}
		if seen {
			empty = false
			continue
		}
		if term.Required {
			missingRequired = true
		}
	}
	if empty && model.Empty {
		return nil
	}
	if empty || missingRequired {
		return validation(ErrValidationContent, line, col, s.pathString(), "missing required child element")
	}
	return nil
}

func (s *session) completeDFAModel(f *frame, model compiledModel, line, col int) error {
	row, err := s.dfaRow(model, f.State)
	if err != nil {
		return err
	}
	if row.Accept && (!row.Counted || f.Count >= row.Min) {
		return nil
	}
	return validation(ErrValidationContent, line, col, s.pathString(), "missing required child element")
}

func (s *session) dfaRow(model compiledModel, state uint32) (compiledModelRow, error) {
	if !validUint32Index(state, len(model.Rows)) {
		return compiledModelRow{}, s.counterInvariantError("content model DFA state out of range", int(state), len(model.Rows))
	}
	return model.Rows[state], nil
}

func (s *session) advanceDFA(f *frame, model compiledModel, edge compiledModelEdge) (bool, error) {
	to := edge.To
	if !validUint32Index(to, len(model.Rows)) {
		return false, s.counterInvariantError("content model DFA state out of range", int(to), len(model.Rows))
	}
	from := model.Rows[f.State]
	next := model.Rows[to]
	count := uint32(0)
	switch {
	case from.Counted && to == f.State && sameCompiledParticle(edge.Particle, from.CountParticle):
		if !from.Unbounded && f.Count >= from.Max {
			return false, nil
		}
		count = f.Count + 1
	default:
		if from.Counted && f.Count < from.Min {
			return false, nil
		}
		if next.Counted && sameCompiledParticle(edge.Particle, next.CountParticle) {
			count = 1
		}
	}
	f.State = to
	f.Count = count
	return true, nil
}

func (s *session) allSeen(f *frame, i int) (bool, error) {
	if i < 0 || i/64 >= f.BitLen {
		return false, s.counterInvariantError("content model all bit index out of range", i, f.BitLen*64)
	}
	idx := f.BitBase + i/64
	if idx < 0 || idx >= len(s.allBits) {
		return false, s.counterInvariantError("content model all bit storage out of range", idx, len(s.allBits))
	}
	return s.allBits[idx]&(uint64(1)<<uint(i%64)) != 0, nil
}

func (s *session) setAllSeen(f *frame, i int) error {
	if i < 0 || i/64 >= f.BitLen {
		return s.counterInvariantError("content model all bit index out of range", i, f.BitLen*64)
	}
	idx := f.BitBase + i/64
	if idx < 0 || idx >= len(s.allBits) {
		return s.counterInvariantError("content model all bit storage out of range", idx, len(s.allBits))
	}
	s.allBits[idx] |= uint64(1) << uint(i%64)
	return nil
}

func (s *session) counterInvariantError(msg string, got, limit int) error {
	return &Error{
		Category: InternalErrorCategory,
		Code:     ErrInternalInvariant,
		Path:     s.pathString(),
		Message:  fmt.Sprintf("%s: %d not in [0,%d)", msg, got, limit),
	}
}
