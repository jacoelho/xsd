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

func skippedAnyTypeChild(rt *runtimeSchema) acceptedChild {
	return acceptedChild{element: noElement, typ: anyType(rt), skip: true}
}

type acceptedChild struct {
	element elementID
	typ     typeID
	skip    bool
}

func (s *session) acceptChild(parent *frame, rn runtimeName, attrs []streamAttr, line, col int) (acceptedChild, error) {
	rt := s.engine.rt
	if parent.Skip {
		return skippedAnyTypeChild(rt), nil
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
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "unexpected child element "+rn.label())
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
		match, ok = s.acceptDFAChild(parent, model, rn, attrs)
	default:
		ok = false
	}
	if err != nil {
		return acceptedChild{}, err
	}
	if !ok {
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "unexpected child element "+rn.label())
	}
	if match.strictMissing {
		if s.hasSchemaLocationHint(rn.NS) {
			return acceptedChild{}, s.unsupportedSchemaLocation(line, col, xsdElemElement, rn)
		}
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "wildcard requires declared element "+rn.label())
	}
	return match.child(rt), nil
}

func (s *session) acceptAny(rn runtimeName, w wildcard, line, col int) (acceptedChild, error) {
	rt := s.engine.rt
	if !wildcardMatches(rt, w, rn) {
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "element is not allowed by wildcard "+rn.label())
	}
	if rn.Known {
		if id, ok := rt.GlobalElements[rn.Name]; ok && w.Process != processSkip {
			return acceptedChild{element: id, typ: rt.Elements[id].Type}, nil
		}
	}
	if w.Process == processSkip {
		return skippedAnyTypeChild(rt), nil
	}
	if w.Process == processStrict {
		if s.hasSchemaLocationHint(rn.NS) {
			return acceptedChild{}, s.unsupportedSchemaLocation(line, col, xsdElemElement, rn)
		}
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "wildcard requires declared element "+rn.label())
	}
	return acceptedChild{element: noElement, typ: anyType(rt)}, nil
}

func (s *session) acceptAllChild(f *frame, model compiledModel, rn runtimeName, attrs []streamAttr) (matchResult, bool, error) {
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

func (s *session) acceptDFAChild(f *frame, model compiledModel, rn runtimeName, attrs []streamAttr) (matchResult, bool) {
	row := model.Rows[f.State]
	for _, edge := range row.Edges {
		match, matched := s.matchDirectParticle(edge.Particle, rn, attrs)
		if !matched {
			continue
		}
		if !s.advanceDFA(f, model, edge) {
			continue
		}
		return match, true
	}
	return noMatch(), false
}

func (s *session) matchDirectParticle(p particle, rn runtimeName, attrs []streamAttr) (matchResult, bool) {
	rt := s.engine.rt
	switch p.Kind {
	case particleElement:
		if rn.Known && rt.Elements[p.Element].Name == rn.Name {
			return matchResult{element: p.Element}, true
		}
		if rn.Known {
			if byName := rt.SubstitutionLookup[p.Element]; byName != nil {
				if member, ok := byName[rn.Name]; ok {
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
	case particleModel:
	}
	return noMatch(), false
}

func hasXSIType(attrs []streamAttr) bool {
	for _, a := range attrs {
		if a.Name.Space == xsiNamespaceURI && a.Name.Local == xsiAttrType {
			return true
		}
	}
	return false
}

func (s *session) end(line, col int, ee xml.EndElement) error {
	if len(s.stack) == 0 {
		return validation(ErrValidationXML, line, col, s.pathString(), "unexpected end element")
	}
	translated, err := s.translateName(ee.Name, xmlElementName, line, col)
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
	s.popPath()
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
	contentCaptured, err := s.validateSimpleContent(f, line, col)
	if err != nil {
		return s.recover(err)
	}
	if len(s.engine.rt.Identities) == 0 {
		return nil
	}
	return s.captureEndIdentity(f, contentCaptured, line, col)
}

func (s *session) captureEndIdentity(f *frame, contentCaptured bool, line, col int) error {
	switch {
	case contentCaptured:
		return nil
	case f.Nilled && f.Element != noElement:
		return s.recover(s.captureIdentityFields(s.identityElementFields(), nilledIdentityValue(), line, col))
	case f.Type.Kind == typeComplex && !s.engine.rt.ComplexTypes[f.Type.ID].SimpleValue:
		return s.recover(s.captureIdentityComplexElement(s.text[f.TextStart:], line, col))
	default:
		return nil
	}
}

func (s *session) finishFrameIdentity(line, col int) error {
	if len(s.engine.rt.Identities) == 0 {
		return nil
	}
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
	switch model.Kind {
	case compiledModelEmpty, compiledModelAny:
		return nil
	case compiledModelAll:
		return s.completeAllModel(f, model, line, col)
	case compiledModelDFA:
		return s.completeDFAModel(f, model, line, col)
	default:
		return &Error{Category: InternalErrorCategory, Code: ErrInternalInvariant, Line: line, Column: col, Path: s.pathString(), Message: "compiled content model has invalid kind"}
	}
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
	if !validUint32Index(f.State, len(model.Rows)) {
		return s.counterInvariantError("content model DFA state out of range", int(f.State), len(model.Rows))
	}
	row := model.Rows[f.State]
	if row.Accept && (!row.Counted || f.Count >= row.Min) {
		return nil
	}
	return validation(ErrValidationContent, line, col, s.pathString(), "missing required child element")
}

func (s *session) advanceDFA(f *frame, model compiledModel, edge compiledModelEdge) bool {
	to := edge.To
	from := model.Rows[f.State]
	next := model.Rows[to]
	count := uint32(0)
	if from.Counted && to == f.State && sameCompiledParticle(edge.Particle, from.CountParticle) {
		if !from.Unbounded && f.Count >= from.Max {
			return false
		}
		count = f.Count + 1
	} else {
		if from.Counted && f.Count < from.Min {
			return false
		}
		if next.Counted && sameCompiledParticle(edge.Particle, next.CountParticle) {
			count = 1
		}
	}
	f.State = to
	f.Count = count
	return true
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
