package xsd

import (
	"encoding/xml"
	"slices"
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
		return acceptedChild{element: noElement, typ: typeID{Kind: typeComplex, ID: uint32(rt.Builtin.AnyType)}, skip: m.skip}
	}
	decl := rt.Elements[m.element]
	return acceptedChild{element: m.element, typ: decl.Type, skip: m.skip}
}

func anyTypeChild(rt *runtimeSchema, skip bool) acceptedChild {
	return acceptedChild{element: noElement, typ: typeID{Kind: typeComplex, ID: uint32(rt.Builtin.AnyType)}, skip: skip}
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
	model := rt.Models[parent.Model]
	if model.Kind == modelAny {
		return s.acceptAny(rn, wildcard{Mode: wildAny, Process: processLax}, line, col)
	}
	match, ok := s.matchModel(parent, parent.Model, rn, attrs)
	if !ok {
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "unexpected child element "+rn.Local)
	}
	if match.strictMissing {
		if s.hasSchemaLocationHint(rn.NS) {
			return acceptedChild{}, s.unsupportedSchemaLocation(line, col, "element", rn)
		}
		return acceptedChild{}, validation(ErrValidationElement, line, col, s.pathString(), "wildcard requires declared element "+rn.Local)
	}
	if rt.Models[parent.Model].Replay {
		parent.Children = append(parent.Children, rn)
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

func (s *session) matchModel(f *frame, modelID contentModelID, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	return s.matchModelState(f, modelID, 0, rn, attrs)
}

func (s *session) matchModelState(f *frame, modelID contentModelID, base int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	rt := s.engine.rt
	model := rt.Models[modelID]
	switch model.Kind {
	case modelEmpty:
		return noMatch(), false
	case modelSequence:
		return s.matchSequenceState(f, modelID, base, rn, attrs)
	case modelChoice:
		return s.matchChoiceState(f, modelID, base, rn, attrs)
	case modelAll:
		return s.matchAllState(f, modelID, base, rn, attrs)
	}
	return noMatch(), false
}

func (s *session) matchSequenceState(f *frame, modelID contentModelID, base int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	model := s.engine.rt.Models[modelID]
	for {
		if !model.occurs.isExactlyOne() && !s.modelCurrentProgress(f, model, base) && !model.occurs.canAdd(s.modelOccurs(f, base)) {
			return noMatch(), false
		}
		if s.shouldStartNextSequenceOccurrenceState(f, modelID, base, rn, attrs) {
			s.finishModelOccurrence(f, modelID, base)
			continue
		}
		index := s.modelIndex(f, base)
		for index < len(model.Particles) {
			if match, ok := s.matchSequenceParticleState(f, model, base, index, rn, attrs); ok {
				return match, true
			}
			if s.particleSatisfiedState(f, model, base, index) {
				index++
				s.setModelIndex(f, base, index)
				continue
			}
			return noMatch(), false
		}
		if model.occurs.isExactlyOne() {
			return noMatch(), false
		}
		if !s.modelOccurrenceComplete(f, modelID, base) || !s.modelCurrentProgress(f, model, base) {
			return noMatch(), false
		}
		s.finishModelOccurrence(f, modelID, base)
	}
}

func (s *session) shouldStartNextSequenceOccurrenceState(f *frame, modelID contentModelID, base int, rn runtimeName, attrs []xml.Attr) bool {
	model := s.engine.rt.Models[modelID]
	if model.occurs.isExactlyOne() || !s.modelCurrentProgress(f, model, base) {
		return false
	}
	if s.modelOccurs(f, base)+1 >= model.occurs.Min {
		return false
	}
	if !s.modelOccurrenceComplete(f, modelID, base) || len(model.Particles) == 0 {
		return false
	}
	return s.sequenceWouldMatchAfterFinish(f, modelID, base, rn, attrs)
}

func (s *session) sequenceWouldMatchAfterFinish(f *frame, modelID contentModelID, base int, rn runtimeName, attrs []xml.Attr) bool {
	n := modelCounterLen(s.engine.rt, s.engine.rt.Models[modelID])
	start := len(s.counterScratch)
	for range n {
		s.counterScratch = append(s.counterScratch, 0)
	}
	snapshot := s.counterScratch[start:]
	copy(snapshot, s.counters[f.CounterBase+base:f.CounterBase+base+n])
	s.finishModelOccurrence(f, modelID, base)
	model := s.engine.rt.Models[modelID]
	_, ok := s.matchSequenceParticleState(f, model, base, 0, rn, attrs)
	copy(s.counters[f.CounterBase+base:f.CounterBase+base+n], snapshot)
	s.counterScratch = s.counterScratch[:start]
	return ok
}

func (s *session) matchSequenceParticleState(f *frame, model contentModel, base, i int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	p := model.Particles[i]
	if p.Kind == particleModel {
		return s.matchParticleModelState(f, model, base, i, rn, attrs)
	}
	count := s.particleCount(f, base, i)
	if !p.occurs.canAdd(count) {
		return noMatch(), false
	}
	match, ok := s.matchDirectParticle(p, rn, attrs)
	if ok {
		s.setParticleCount(f, base, i, count+1)
	}
	return match, ok
}

func (s *session) matchChoiceState(f *frame, modelID contentModelID, base int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	model := s.engine.rt.Models[modelID]
	for {
		if !model.occurs.isExactlyOne() && !s.modelCurrentProgress(f, model, base) && !model.occurs.canAdd(s.modelOccurs(f, base)) {
			return noMatch(), false
		}
		selected := s.modelIndex(f, base)
		if selected != 0 {
			i := selected - 1
			if i < 0 || i >= len(model.Particles) {
				return noMatch(), false
			}
			if match, ok := s.matchChoiceParticleState(f, model, base, i, rn, attrs); ok {
				return match, true
			}
			if !s.particleSatisfiedState(f, model, base, i) || model.occurs.isExactlyOne() {
				return noMatch(), false
			}
			s.finishModelOccurrence(f, modelID, base)
			continue
		}
		for i := range model.Particles {
			if match, ok := s.matchChoiceParticleState(f, model, base, i, rn, attrs); ok {
				s.setModelIndex(f, base, i+1)
				return match, true
			}
		}
		return noMatch(), false
	}
}

func (s *session) matchChoiceParticleState(f *frame, model contentModel, base, i int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	p := model.Particles[i]
	if p.Kind == particleModel {
		return s.matchParticleModelState(f, model, base, i, rn, attrs)
	}
	count := s.particleCount(f, base, i)
	if !p.occurs.canAdd(count) {
		return noMatch(), false
	}
	match, ok := s.matchDirectParticle(p, rn, attrs)
	if ok {
		s.setParticleCount(f, base, i, count+1)
	}
	return match, ok
}

func (s *session) matchAllState(f *frame, modelID contentModelID, base int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	model := s.engine.rt.Models[modelID]
	if !model.occurs.isExactlyOne() && !s.modelCurrentProgress(f, model, base) && !model.occurs.canAdd(s.modelOccurs(f, base)) {
		return noMatch(), false
	}
	for i := range model.Particles {
		if match, ok := s.matchAllParticleState(f, model, base, i, rn, attrs); ok {
			return match, true
		}
	}
	return noMatch(), false
}

func (s *session) matchAllParticleState(f *frame, model contentModel, base, i int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	p := model.Particles[i]
	if p.Kind == particleModel {
		return s.matchParticleModelState(f, model, base, i, rn, attrs)
	}
	count := s.particleCount(f, base, i)
	if !p.occurs.canAdd(count) {
		return noMatch(), false
	}
	match, ok := s.matchDirectParticle(p, rn, attrs)
	if ok {
		s.setParticleCount(f, base, i, count+1)
	}
	return match, ok
}

func (s *session) matchParticleModelState(f *frame, parent contentModel, parentBase, i int, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	p := parent.Particles[i]
	childBase := modelChildBase(s.engine.rt, parent, parentBase, i)
	child := s.engine.rt.Models[p.Model]
	count := s.particleCount(f, parentBase, i)
	if s.modelCurrentProgress(f, child, childBase) {
		if s.modelOccurrenceComplete(f, p.Model, childBase) && count < p.occurs.Min && p.occurs.canAdd(count) {
			if match, ok := s.restartParticleModelState(f, parent, parentBase, i, childBase, count, rn, attrs); ok {
				return match, true
			}
		}
		if match, ok := s.withModelSnapshot(f, p.Model, childBase, func() (matchResult, bool) {
			return s.matchModelState(f, p.Model, childBase, rn, attrs)
		}); ok {
			return match, true
		}
		if s.modelOccurrenceComplete(f, p.Model, childBase) && p.occurs.canAdd(count) {
			return s.restartParticleModelState(f, parent, parentBase, i, childBase, count, rn, attrs)
		}
		return noMatch(), false
	}
	if !p.occurs.canAdd(count) {
		return noMatch(), false
	}
	match, ok := s.withModelSnapshot(f, p.Model, childBase, func() (matchResult, bool) {
		return s.matchModelState(f, p.Model, childBase, rn, attrs)
	})
	if ok {
		s.setParticleCount(f, parentBase, i, count+1)
	}
	return match, ok
}

func (s *session) restartParticleModelState(f *frame, parent contentModel, parentBase, i, childBase int, count uint32, rn runtimeName, attrs []xml.Attr) (matchResult, bool) {
	p := parent.Particles[i]
	return s.withModelSnapshot(f, p.Model, childBase, func() (matchResult, bool) {
		s.resetModelState(f, p.Model, childBase)
		match, ok := s.matchModelState(f, p.Model, childBase, rn, attrs)
		if ok {
			s.setParticleCount(f, parentBase, i, count+1)
		}
		return match, ok
	})
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

func (s *session) withModelSnapshot(f *frame, modelID contentModelID, base int, fn func() (matchResult, bool)) (matchResult, bool) {
	n := modelCounterLen(s.engine.rt, s.engine.rt.Models[modelID])
	start := len(s.counterScratch)
	for range n {
		s.counterScratch = append(s.counterScratch, 0)
	}
	snapshot := s.counterScratch[start:]
	copy(snapshot, s.counters[f.CounterBase+base:f.CounterBase+base+n])
	match, ok := fn()
	if !ok {
		copy(s.counters[f.CounterBase+base:f.CounterBase+base+n], snapshot)
	}
	s.counterScratch = s.counterScratch[:start]
	return match, ok
}

func (s *session) modelIndex(f *frame, base int) int {
	return int(s.counter(f, base))
}

func (s *session) setModelIndex(f *frame, base int, v int) {
	s.setCounter(f, base, uint32(v))
}

func (s *session) modelOccurs(f *frame, base int) uint32 {
	return s.counter(f, base+1)
}

func (s *session) setModelOccurs(f *frame, base int, v uint32) {
	s.setCounter(f, base+1, v)
}

func (s *session) particleCount(f *frame, base, i int) uint32 {
	return s.counter(f, base+modelStateHeaderLen+i)
}

func (s *session) setParticleCount(f *frame, base, i int, v uint32) {
	s.setCounter(f, base+modelStateHeaderLen+i, v)
}

func (s *session) modelCurrentProgress(f *frame, model contentModel, base int) bool {
	for i := range model.Particles {
		if s.particleCount(f, base, i) != 0 {
			return true
		}
	}
	return false
}

func (s *session) finishModelOccurrence(f *frame, modelID contentModelID, base int) {
	count := s.modelOccurs(f, base)
	s.resetModelState(f, modelID, base)
	s.setModelOccurs(f, base, count+1)
}

func (s *session) resetModelState(f *frame, modelID contentModelID, base int) {
	n := modelCounterLen(s.engine.rt, s.engine.rt.Models[modelID])
	for i := range n {
		s.setCounter(f, base+i, 0)
	}
}

func (s *session) modelOccurrenceComplete(f *frame, modelID contentModelID, base int) bool {
	model := s.engine.rt.Models[modelID]
	switch model.Kind {
	case modelEmpty, modelAny:
		return true
	case modelSequence:
		for i := s.modelIndex(f, base); i < len(model.Particles); i++ {
			if !s.particleSatisfiedState(f, model, base, i) {
				return false
			}
		}
		return true
	case modelChoice:
		selected := s.modelIndex(f, base)
		if selected != 0 {
			i := selected - 1
			return i >= 0 && i < len(model.Particles) && s.particleSatisfiedState(f, model, base, i)
		}
		for i, p := range model.Particles {
			if p.occurs.Min == 0 || s.particleTermEmptiableState(model, i) {
				return true
			}
		}
		return len(model.Particles) == 0 && model.occurs.Min == 0
	case modelAll:
		for i := range model.Particles {
			if !s.particleSatisfiedState(f, model, base, i) {
				return false
			}
		}
		return true
	}
	return false
}

func (s *session) particleSatisfiedState(f *frame, model contentModel, base, i int) bool {
	p := model.Particles[i]
	count := s.particleCount(f, base, i)
	if count < p.occurs.Min && !s.particleTermEmptiableState(model, i) {
		return false
	}
	if p.Kind != particleModel || count == 0 {
		return true
	}
	childBase := modelChildBase(s.engine.rt, model, base, i)
	return s.modelOccurrenceComplete(f, p.Model, childBase)
}

func (s *session) particleTermEmptiableState(model contentModel, i int) bool {
	p := model.Particles[i]
	if p.Kind != particleModel {
		return false
	}
	return s.modelEmptiable(p.Model)
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

func (s *session) counter(f *frame, i int) uint32 {
	if i < 0 || i >= f.CounterLen {
		return 0
	}
	return s.counters[f.CounterBase+i]
}

func (s *session) setCounter(f *frame, i int, v uint32) {
	if i < 0 || i >= f.CounterLen {
		return
	}
	s.counters[f.CounterBase+i] = v
}

const modelStateHeaderLen = 2

func modelCounterLen(rt *runtimeSchema, model contentModel) int {
	n := modelStateHeaderLen + len(model.Particles)
	for _, p := range model.Particles {
		if p.Kind == particleModel {
			n += modelCounterLen(rt, rt.Models[p.Model])
		}
	}
	return n
}

func modelChildBase(rt *runtimeSchema, model contentModel, base, particleIndex int) int {
	childBase := base + modelStateHeaderLen + len(model.Particles)
	for i := range particleIndex {
		p := model.Particles[i]
		if p.Kind == particleModel {
			childBase += modelCounterLen(rt, rt.Models[p.Model])
		}
	}
	return childBase
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
	if !f.Skip {
		if f.Nilled && (f.HasChild || f.HasText) {
			return validation(ErrValidationNil, line, col, s.pathString(), "nilled element must be empty")
		}
		if !f.Nilled {
			if err := s.completeFrame(f, line, col); err != nil {
				return err
			}
		}
		canon, typeID, captured, err := s.validateSimpleContent(f, line, col)
		if err != nil {
			return err
		}
		if captured {
			if err := s.captureIdentityElement(typeID, canon, line, col); err != nil {
				return err
			}
		} else if f.Nilled && f.Element != noElement {
			if err := s.captureIdentityElement(noSimpleType, "\x00nil", line, col); err != nil {
				return err
			}
		} else if f.Type.Kind == typeComplex && !s.engine.rt.ComplexTypes[f.Type.ID].SimpleValue {
			rawText := string(s.text[f.TextStart:])
			if err := s.captureIdentityComplexElement(rawText, line, col); err != nil {
				return err
			}
		}
	}
	if err := s.finishIdentitySelections(len(s.namePath), line, col); err != nil {
		return err
	}
	if err := s.closeIdentityScopes(len(s.namePath)); err != nil {
		return err
	}
	s.counters = s.counters[:f.CounterBase]
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
	return nil
}

func (s *session) completeFrame(f *frame, line, col int) error {
	if f.Type.Kind != typeComplex || f.Model == noContentModel {
		return nil
	}
	err := s.completeModelState(f, f.Model, 0, line, col)
	if err != nil && s.engine.rt.Models[f.Model].Replay && s.replayModelAccepts(f.Model, f.Children) {
		return s.validateRestrictionCountLimits(f, line, col)
	}
	if err != nil {
		return err
	}
	return s.validateRestrictionCountLimits(f, line, col)
}

func (s *session) validateRestrictionCountLimits(f *frame, line, col int) error {
	if f.Type.Kind != typeComplex {
		return nil
	}
	limits := s.engine.rt.ComplexTypes[f.Type.ID].CountLimits
	if len(limits) == 0 {
		return nil
	}
	model := s.engine.rt.Models[f.Model]
	for _, limit := range limits {
		i := int(limit.particle)
		if i < 0 || i >= len(model.Particles) {
			return &Error{Category: InternalErrorCategory, Code: ErrInternalInvariant, Line: line, Column: col, Path: s.pathString(), Message: "restriction count limit references missing particle"}
		}
		if s.particleCount(f, 0, i) > limit.Max {
			return validation(ErrValidationContent, line, col, s.pathString(), "content restriction occurrence is not subset of base")
		}
	}
	return nil
}

func (s *session) completeModelState(f *frame, modelID contentModelID, base, line, col int) error {
	model := s.engine.rt.Models[modelID]
	if model.Kind == modelEmpty || model.Kind == modelAny {
		return nil
	}
	progress := s.modelCurrentProgress(f, model, base)
	if !progress && (model.occurs.Min == 0 || s.modelEmptiable(modelID)) {
		return nil
	}
	if progress && !s.modelOccurrenceComplete(f, modelID, base) {
		return validation(ErrValidationContent, line, col, s.pathString(), "missing required child element")
	}
	completed := s.modelOccurs(f, base)
	if progress {
		completed++
	} else if s.modelEmptiable(modelID) {
		completed = model.occurs.Min
	}
	if !model.occurs.allows(completed) {
		return validation(ErrValidationContent, line, col, s.pathString(), "missing required child element")
	}
	return nil
}

func (s *session) modelEmptiable(modelID contentModelID) bool {
	if modelID == noContentModel {
		return true
	}
	model := s.engine.rt.Models[modelID]
	if model.occurs.Min == 0 {
		return true
	}
	switch model.Kind {
	case modelEmpty, modelAny:
		return true
	case modelSequence, modelAll:
		for _, p := range model.Particles {
			if p.occurs.Min > 0 && !s.particleTermEmptiable(p) {
				return false
			}
		}
		return true
	case modelChoice:
		for _, p := range model.Particles {
			if p.occurs.Min == 0 || s.particleTermEmptiable(p) {
				return true
			}
		}
	}
	return false
}

func (s *session) particleTermEmptiable(p particle) bool {
	if p.Kind != particleModel {
		return false
	}
	return s.modelEmptiable(p.Model)
}

type replayKey struct {
	Model contentModelID
	Pos   int
}

func (s *session) replayModelAccepts(modelID contentModelID, names []runtimeName) bool {
	memo := make(map[replayKey][]int)
	return slices.Contains(s.replayModel(modelID, names, 0, memo), len(names))
}

func (s *session) replayModel(modelID contentModelID, names []runtimeName, pos int, memo map[replayKey][]int) []int {
	key := replayKey{Model: modelID, Pos: pos}
	if ends, ok := memo[key]; ok {
		return ends
	}
	memo[key] = nil
	model := s.engine.rt.Models[modelID]
	ends := s.replayRepeat(model.occurs, names, pos, func(p int) []int {
		return s.replayModelOnce(model, names, p, memo)
	})
	memo[key] = ends
	return ends
}

func (s *session) replayModelOnce(model contentModel, names []runtimeName, pos int, memo map[replayKey][]int) []int {
	switch model.Kind {
	case modelEmpty:
		return []int{pos}
	case modelAny:
		if pos < len(names) {
			return []int{pos + 1}
		}
		return nil
	case modelSequence:
		positions := []int{pos}
		for _, p := range model.Particles {
			var next []int
			for _, current := range positions {
				next = addReplayPositions(next, s.replayParticle(p, names, current, memo))
			}
			if len(next) == 0 {
				return nil
			}
			positions = next
		}
		return positions
	case modelChoice:
		var out []int
		for _, p := range model.Particles {
			out = addReplayPositions(out, s.replayParticle(p, names, pos, memo))
		}
		return out
	case modelAll:
		return s.replayAll(model, names, pos, 0, memo)
	}
	return nil
}

func (s *session) replayParticle(p particle, names []runtimeName, pos int, memo map[replayKey][]int) []int {
	return s.replayRepeat(p.occurs, names, pos, func(current int) []int {
		switch p.Kind {
		case particleElement:
			if current < len(names) && s.replayElementMatches(p.Element, names[current]) {
				return []int{current + 1}
			}
		case particleWildcard:
			if current < len(names) && wildcardMatches(s.engine.rt, s.engine.rt.Wildcards[p.wildcard], names[current]) {
				return []int{current + 1}
			}
		case particleModel:
			return s.replayModel(p.Model, names, current, memo)
		}
		return nil
	})
}

func (s *session) replayAll(model contentModel, names []runtimeName, pos int, used uint64, memo map[replayKey][]int) []int {
	complete := true
	for i, p := range model.Particles {
		if used&(uint64(1)<<uint(i)) == 0 && p.occurs.Min > 0 && !s.particleTermEmptiable(p) {
			complete = false
			break
		}
	}
	var out []int
	if complete {
		out = append(out, pos)
	}
	for i, p := range model.Particles {
		if used&(uint64(1)<<uint(i)) != 0 {
			continue
		}
		for _, end := range s.replayParticle(p, names, pos, memo) {
			if end == pos {
				continue
			}
			out = addReplayPositions(out, s.replayAll(model, names, end, used|(uint64(1)<<uint(i)), memo))
		}
	}
	return out
}

func (s *session) replayRepeat(occ occurrence, names []runtimeName, pos int, once func(int) []int) []int {
	positions := []int{pos}
	var out []int
	limit := occ.Max
	if occ.Unbounded || limit > uint32(len(names))+occ.Min+1 {
		limit = uint32(len(names)) + occ.Min + 1
	}
	for count := uint32(0); ; count++ {
		if count >= occ.Min {
			out = addReplayPositions(out, positions)
		}
		if count >= limit {
			break
		}
		var next []int
		for _, current := range positions {
			for _, end := range once(current) {
				if end == current && count >= occ.Min {
					continue
				}
				next = addReplayPosition(next, end)
			}
		}
		if len(next) == 0 {
			break
		}
		positions = next
	}
	return out
}

func (s *session) replayElementMatches(element elementID, rn runtimeName) bool {
	rt := s.engine.rt
	if rn.Known && rt.Elements[element].Name == rn.Name {
		return true
	}
	if !rn.Known {
		return false
	}
	for _, member := range rt.Substitutions[element] {
		if rt.Elements[member].Name == rn.Name && s.substitutionAllowed(element, member) {
			return true
		}
	}
	return false
}

func addReplayPositions(out, positions []int) []int {
	for _, pos := range positions {
		out = addReplayPosition(out, pos)
	}
	return out
}

func addReplayPosition(out []int, pos int) []int {
	if slices.Contains(out, pos) {
		return out
	}
	return append(out, pos)
}
