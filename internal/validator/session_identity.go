package validator

import (
	"bytes"
	"fmt"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/ic"
	"github.com/jacoelho/xsd/internal/runtime"
	xsdxml "github.com/jacoelho/xsd/internal/xml"
)

type identityState struct {
	arena      *Arena
	frames     []rtIdentityFrame
	scopes     []rtIdentityScope
	violations []error
	pending    []error
	nextNodeID uint64
	active     bool
}

type identityStartInput struct {
	Attrs   []StartAttr
	Applied []AttrApplied
	Elem    runtime.ElemID
	Type    runtime.TypeID
	Sym     runtime.SymbolID
	NS      runtime.NamespaceID
	Nilled  bool
}

type identityEndInput struct {
	Text      []byte
	KeyBytes  []byte
	TextState TextState
	KeyKind   runtime.ValueKind
}

type rtIdentityFrame struct {
	captures []rtFieldCapture
	matches  []*rtSelectorMatch
	id       uint64
	depth    int
	sym      runtime.SymbolID
	ns       runtime.NamespaceID
	elem     runtime.ElemID
	typ      runtime.TypeID
	nilled   bool
}

type rtFieldNodeKind int

const (
	rtFieldNodeElement rtFieldNodeKind = iota
	rtFieldNodeAttribute
)

type rtFieldNodeKey struct {
	attrKey string
	kind    rtFieldNodeKind
	elemID  uint64
	attrSym runtime.SymbolID
}

type rtFieldCapture struct {
	match      *rtSelectorMatch
	fieldIndex int
}

type rtFieldState struct {
	nodes    map[rtFieldNodeKey]struct{}
	keyBytes []byte
	count    int
	keyKind  runtime.ValueKind
	multiple bool
	missing  bool
	invalid  bool
	hasValue bool
}

func (s *rtFieldState) addNode(key rtFieldNodeKey) bool {
	if s.nodes == nil {
		s.nodes = make(map[rtFieldNodeKey]struct{})
	}
	if _, ok := s.nodes[key]; ok {
		return false
	}
	s.nodes[key] = struct{}{}
	s.count++
	if s.count > 1 {
		s.multiple = true
	}
	return true
}

type rtSelectorMatch struct {
	constraint *rtConstraintState
	fields     []rtFieldState
	id         uint64
	depth      int
	invalid    bool
}

type rtConstraintState struct {
	matches    map[uint64]*rtSelectorMatch
	selectors  []runtime.PathID
	fields     [][]runtime.PathID
	rows       []rtIdentityRow
	keyrefRows []rtIdentityRow
	violations []error
	id         runtime.ICID
	referenced runtime.ICID
	category   runtime.ICCategory
}

type rtIdentityRow struct {
	values []runtime.ValueKey
	hash   uint64
}

type rtIdentityScope struct {
	constraints []rtConstraintState
	rootID      uint64
	rootDepth   int
	rootElem    runtime.ElemID
}

type rtIdentityAttr struct {
	nsBytes  []byte
	local    []byte
	keyBytes []byte
	sym      runtime.SymbolID
	ns       runtime.NamespaceID
	keyKind  runtime.ValueKind
}

func (s *Session) identityStart(in identityStartInput) error {
	if s == nil {
		return nil
	}
	prevFrames := len(s.icState.frames)
	prevScopes := len(s.icState.scopes)
	prevViolations := len(s.icState.violations)
	prevPending := len(s.icState.pending)
	prevNodeID := s.icState.nextNodeID
	prevActive := s.icState.active
	err := s.icState.start(s.rt, in)
	if err != nil {
		s.icState.frames = s.icState.frames[:prevFrames]
		s.icState.scopes = s.icState.scopes[:prevScopes]
		s.icState.violations = s.icState.violations[:prevViolations]
		s.icState.pending = s.icState.pending[:prevPending]
		s.icState.nextNodeID = prevNodeID
		s.icState.active = prevActive
	}
	return err
}

func (s *Session) identityEnd(in identityEndInput) error {
	if s == nil {
		return nil
	}
	return s.icState.end(s.rt, in)
}

func (s *identityState) reset() {
	if s == nil {
		return
	}
	s.active = false
	s.nextNodeID = 0
	s.frames = s.frames[:0]
	s.scopes = s.scopes[:0]
	s.violations = s.violations[:0]
	s.pending = s.pending[:0]
}

func (s *identityState) start(rt *runtime.Schema, in identityStartInput) error {
	if rt == nil {
		return fmt.Errorf("identity: schema missing")
	}
	elem, ok := elementByID(rt, in.Elem)
	if !ok {
		return fmt.Errorf("identity: element %d not found", in.Elem)
	}
	hasConstraints := elem.ICLen > 0
	if !s.active && !hasConstraints {
		return nil
	}
	s.active = true

	s.nextNodeID++
	frame := rtIdentityFrame{
		id:     s.nextNodeID,
		depth:  len(s.frames),
		sym:    in.Sym,
		ns:     in.NS,
		elem:   in.Elem,
		typ:    in.Type,
		nilled: in.Nilled,
	}
	s.frames = append(s.frames, frame)
	current := &s.frames[len(s.frames)-1]

	if hasConstraints {
		if err := s.openScope(rt, current, elem); err != nil {
			return err
		}
	}
	if len(s.scopes) == 0 {
		return nil
	}

	s.matchSelectors(rt, current.depth)
	attrs := collectIdentityAttrs(rt, in.Attrs, in.Applied)
	s.applyFieldSelections(rt, current.depth, attrs)
	return nil
}

func (s *identityState) end(rt *runtime.Schema, in identityEndInput) error {
	if rt == nil || !s.active || len(s.frames) == 0 {
		return nil
	}
	index := len(s.frames) - 1
	frame := &s.frames[index]
	elem, ok := elementByID(rt, frame.elem)
	if !ok {
		return fmt.Errorf("identity: element %d not found", frame.elem)
	}

	s.applyFieldCaptures(frame, elem, in)
	s.finalizeMatches(frame)
	s.closeScopes(frame.id)

	s.frames = s.frames[:index]
	if len(s.frames) == 0 && len(s.scopes) == 0 {
		s.active = false
	}
	return nil
}

func (s *identityState) openScope(rt *runtime.Schema, frame *rtIdentityFrame, elem *runtime.Element) error {
	if elem == nil {
		return fmt.Errorf("identity: element missing")
	}
	icIDs, err := sliceElemICs(rt, elem)
	if err != nil {
		return err
	}
	if len(icIDs) == 0 {
		return nil
	}
	scope := rtIdentityScope{
		rootID:    frame.id,
		rootDepth: frame.depth,
		rootElem:  frame.elem,
	}
	scope.constraints = make([]rtConstraintState, 0, len(icIDs))
	for _, id := range icIDs {
		if id == 0 || int(id) >= len(rt.ICs) {
			return fmt.Errorf("identity: constraint %d out of range", id)
		}
		constraint := rt.ICs[id]
		selectors, err := slicePathIDs(rt.ICSelectors, constraint.SelectorOff, constraint.SelectorLen)
		if err != nil {
			return err
		}
		fieldsFlat, err := slicePathIDs(rt.ICFields, constraint.FieldOff, constraint.FieldLen)
		if err != nil {
			return err
		}
		fields, err := splitFieldPaths(fieldsFlat)
		if err != nil {
			return err
		}
		scope.constraints = append(scope.constraints, rtConstraintState{
			id:         id,
			category:   constraint.Category,
			referenced: constraint.Referenced,
			selectors:  selectors,
			fields:     fields,
			matches:    make(map[uint64]*rtSelectorMatch),
		})
	}
	s.scopes = append(s.scopes, scope)
	return nil
}

func (s *identityState) matchSelectors(rt *runtime.Schema, currentDepth int) {
	frame := &s.frames[currentDepth]
	for scopeIdx := range s.scopes {
		scope := &s.scopes[scopeIdx]
		for cidx := range scope.constraints {
			state := &scope.constraints[cidx]
			if _, exists := state.matches[frame.id]; exists {
				continue
			}
			if !matchesAnySelector(rt, state.selectors, s.frames, scope.rootDepth, currentDepth) {
				continue
			}
			match := &rtSelectorMatch{
				constraint: state,
				id:         frame.id,
				depth:      currentDepth,
				fields:     make([]rtFieldState, len(state.fields)),
			}
			state.matches[frame.id] = match
			frame.matches = append(frame.matches, match)
		}
	}
}

func (s *identityState) applyFieldSelections(rt *runtime.Schema, currentDepth int, attrs []rtIdentityAttr) {
	frame := &s.frames[currentDepth]
	for scopeIdx := range s.scopes {
		scope := &s.scopes[scopeIdx]
		for cidx := range scope.constraints {
			state := &scope.constraints[cidx]
			for _, match := range state.matches {
				if match.invalid {
					continue
				}
				for fieldIndex := range match.fields {
					fieldState := &match.fields[fieldIndex]
					if fieldState.multiple {
						continue
					}
					for _, pathID := range state.fields[fieldIndex] {
						ops, ok := pathOps(rt, pathID)
						if !ok {
							s.violations = append(s.violations, fmt.Errorf("identity: path %d out of range", pathID))
							continue
						}
						elemOps, attrOp, hasAttr := splitAttrOp(ops)
						if !matchProgramPath(elemOps, s.frames, match.depth, currentDepth) {
							continue
						}
						if hasAttr {
							s.applyAttributeSelection(fieldState, attrOp, attrs, frame, rt)
						} else {
							s.applyElementSelection(fieldState, frame, match, fieldIndex, rt)
						}
						if fieldState.multiple {
							break
						}
					}
				}
			}
		}
	}
}

func (s *identityState) applyElementSelection(state *rtFieldState, frame *rtIdentityFrame, match *rtSelectorMatch, fieldIndex int, rt *runtime.Schema) {
	if state.multiple {
		return
	}
	key := rtFieldNodeKey{kind: rtFieldNodeElement, elemID: frame.id}
	if !state.addNode(key) {
		return
	}
	if state.count > 1 {
		state.multiple = true
		return
	}
	if !isSimpleContent(rt, frame.typ) {
		state.invalid = true
		return
	}
	frame.captures = append(frame.captures, rtFieldCapture{match: match, fieldIndex: fieldIndex})
}

func (s *identityState) applyAttributeSelection(state *rtFieldState, op runtime.PathOp, attrs []rtIdentityAttr, frame *rtIdentityFrame, rt *runtime.Schema) {
	if state.multiple {
		return
	}
	switch op.Op {
	case runtime.OpAttrAny:
		for i := range attrs {
			attr := &attrs[i]
			if isXMLNSAttr(attr, rt) {
				continue
			}
			s.addAttributeValue(state, frame.id, attr)
			if state.multiple {
				return
			}
		}
	case runtime.OpAttrNSAny:
		for i := range attrs {
			attr := &attrs[i]
			if isXMLNSAttr(attr, rt) {
				continue
			}
			if !attrNamespaceMatches(attr, op.NS, rt) {
				continue
			}
			s.addAttributeValue(state, frame.id, attr)
			if state.multiple {
				return
			}
		}
	case runtime.OpAttrName:
		for i := range attrs {
			attr := &attrs[i]
			if isXMLNSAttr(attr, rt) {
				continue
			}
			if attrNameMatches(attr, op, rt) {
				s.addAttributeValue(state, frame.id, attr)
				return
			}
		}
	default:
		s.violations = append(s.violations, fmt.Errorf("identity: unknown attribute op %d", op.Op))
	}
}

func (s *identityState) addAttributeValue(state *rtFieldState, elemID uint64, attr *rtIdentityAttr) {
	if attr == nil {
		return
	}
	if state.multiple {
		return
	}
	key := rtFieldNodeKey{
		kind:    rtFieldNodeAttribute,
		elemID:  elemID,
		attrSym: attr.sym,
	}
	if key.attrSym == 0 {
		key.attrKey = makeAttrKey(attr.nsBytes, attr.local)
	}
	if !state.addNode(key) {
		return
	}
	if state.count > 1 {
		state.multiple = true
		return
	}
	if attr.keyKind == runtime.VKInvalid {
		state.invalid = true
		return
	}
	state.keyKind = attr.keyKind
	state.keyBytes = append(state.keyBytes[:0], attr.keyBytes...)
	state.hasValue = true
}

func (s *identityState) applyFieldCaptures(frame *rtIdentityFrame, elem *runtime.Element, in identityEndInput) {
	kind, key, ok := elementValueKey(frame, elem, in)
	for _, capture := range frame.captures {
		match := capture.match
		if match.invalid {
			continue
		}
		fieldState := &match.fields[capture.fieldIndex]
		if fieldState.multiple || fieldState.invalid {
			continue
		}
		if !ok {
			fieldState.missing = true
			continue
		}
		if kind == runtime.VKInvalid {
			fieldState.invalid = true
			continue
		}
		fieldState.keyKind = kind
		fieldState.keyBytes = append(fieldState.keyBytes[:0], key...)
		fieldState.hasValue = true
	}
}

func (s *identityState) finalizeMatches(frame *rtIdentityFrame) {
	for _, match := range frame.matches {
		if match.invalid {
			delete(match.constraint.matches, match.id)
			continue
		}
		s.finalizeSelectorMatch(match)
		delete(match.constraint.matches, match.id)
	}
}

func (s *identityState) finalizeSelectorMatch(match *rtSelectorMatch) {
	state := match.constraint
	values := make([]runtime.ValueKey, 0, len(match.fields))
	for i := range match.fields {
		field := match.fields[i]
		switch {
		case field.multiple:
			state.violations = append(state.violations, identityViolation(state.category, "identity constraint field selects multiple nodes"))
			return
		case field.count == 0 || field.missing:
			if state.category == runtime.ICUnique {
				return
			}
			state.violations = append(state.violations, identityViolation(state.category, "identity constraint field is missing"))
			return
		case field.invalid || !field.hasValue:
			state.violations = append(state.violations, identityViolation(state.category, "identity constraint field selects non-simple content"))
			return
		default:
			values = append(values, freezeIdentityKey(s.arena, field.keyKind, field.keyBytes))
		}
	}
	row := rtIdentityRow{values: values, hash: ic.HashRow(values)}
	if state.category == runtime.ICKeyRef {
		state.keyrefRows = append(state.keyrefRows, row)
		return
	}
	state.rows = append(state.rows, row)
}

func freezeIdentityKey(arena *Arena, kind runtime.ValueKind, key []byte) runtime.ValueKey {
	if len(key) == 0 {
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, nil)}
	}
	if arena == nil {
		copied := append([]byte(nil), key...)
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, copied), Bytes: copied}
	}
	buf := arena.Alloc(len(key))
	copy(buf, key)
	return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, buf), Bytes: buf}
}

func identityViolation(category runtime.ICCategory, msg string) error {
	switch category {
	case runtime.ICKey:
		return newValidationError(xsderrors.ErrIdentityAbsent, msg)
	case runtime.ICUnique:
		return newValidationError(xsderrors.ErrIdentityDuplicate, msg)
	case runtime.ICKeyRef:
		return newValidationError(xsderrors.ErrIdentityKeyRefFailed, msg)
	default:
		return newValidationError(xsderrors.ErrIdentityAbsent, msg)
	}
}

func (s *identityState) closeScopes(frameID uint64) {
	for i := 0; i < len(s.scopes); {
		scope := &s.scopes[i]
		if scope.rootID != frameID {
			i++
			continue
		}
		s.appendScopeViolations(scope)
		s.scopes = append(s.scopes[:i], s.scopes[i+1:]...)
	}
}

func (s *identityState) appendScopeViolations(scope *rtIdentityScope) {
	if s == nil || scope == nil {
		return
	}
	for i := range scope.constraints {
		if len(scope.constraints[i].violations) > 0 {
			s.pending = append(s.pending, scope.constraints[i].violations...)
		}
	}
	if errs := resolveScopeErrors(scope); len(errs) > 0 {
		s.pending = append(s.pending, errs...)
	}
}

func (s *identityState) drainPending() []error {
	if s == nil || len(s.pending) == 0 {
		return nil
	}
	out := s.pending
	s.pending = s.pending[:0]
	return out
}

func sliceElemICs(rt *runtime.Schema, elem *runtime.Element) ([]runtime.ICID, error) {
	if elem == nil {
		return nil, fmt.Errorf("identity: element missing")
	}
	if elem.ICLen == 0 {
		return nil, nil
	}
	off := elem.ICOff
	end := off + elem.ICLen
	if int(off) > len(rt.ElemICs) || int(end) > len(rt.ElemICs) {
		return nil, fmt.Errorf("identity: elem ICs out of range")
	}
	return rt.ElemICs[off:end], nil
}

func slicePathIDs(list []runtime.PathID, off, ln uint32) ([]runtime.PathID, error) {
	if ln == 0 {
		return nil, fmt.Errorf("identity: empty path list")
	}
	end := off + ln
	if int(off) > len(list) || int(end) > len(list) {
		return nil, fmt.Errorf("identity: path list out of range")
	}
	return list[off:end], nil
}

func splitFieldPaths(ids []runtime.PathID) ([][]runtime.PathID, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("identity: field paths empty")
	}
	hasSep := slices.Contains(ids, 0)
	if !hasSep {
		return [][]runtime.PathID{append([]runtime.PathID(nil), ids...)}, nil
	}
	fields := make([][]runtime.PathID, 0, len(ids))
	cur := make([]runtime.PathID, 0, 4)
	for _, id := range ids {
		if id == 0 {
			if len(cur) == 0 {
				return nil, fmt.Errorf("identity: empty field path set")
			}
			fields = append(fields, cur)
			cur = make([]runtime.PathID, 0, 4)
			continue
		}
		cur = append(cur, id)
	}
	if len(cur) == 0 {
		return nil, fmt.Errorf("identity: trailing field separator")
	}
	fields = append(fields, cur)
	return fields, nil
}

func matchesAnySelector(rt *runtime.Schema, selectors []runtime.PathID, frames []rtIdentityFrame, startDepth, currentDepth int) bool {
	for _, pathID := range selectors {
		ops, ok := pathOps(rt, pathID)
		if !ok {
			continue
		}
		if matchProgramPath(ops, frames, startDepth, currentDepth) {
			return true
		}
	}
	return false
}

func pathOps(rt *runtime.Schema, id runtime.PathID) ([]runtime.PathOp, bool) {
	if id == 0 || int(id) >= len(rt.Paths) {
		return nil, false
	}
	return rt.Paths[id].Ops, true
}

func splitAttrOp(ops []runtime.PathOp) ([]runtime.PathOp, runtime.PathOp, bool) {
	if len(ops) == 0 {
		return nil, runtime.PathOp{}, false
	}
	last := ops[len(ops)-1]
	switch last.Op {
	case runtime.OpAttrName, runtime.OpAttrAny, runtime.OpAttrNSAny:
		return ops[:len(ops)-1], last, true
	default:
		return ops, runtime.PathOp{}, false
	}
}

type stepAxis int

const (
	axisChild stepAxis = iota
	axisSelf
	axisDescendant
	axisDescendantOrSelf
)

type programStep struct {
	axis stepAxis
	op   runtime.PathOp
	any  bool
}

func matchProgramPath(ops []runtime.PathOp, frames []rtIdentityFrame, startDepth, currentDepth int) bool {
	if currentDepth < startDepth || currentDepth >= len(frames) {
		return false
	}
	if len(ops) == 0 {
		return currentDepth == startDepth
	}
	steps := make([]programStep, 0, len(ops))
	start := 0
	if ops[0].Op == runtime.OpDescend {
		steps = append(steps, programStep{axis: axisDescendantOrSelf, any: true})
		start = 1
	}
	for i := start; i < len(ops); i++ {
		op := ops[i]
		switch op.Op {
		case runtime.OpRootSelf, runtime.OpSelf:
			steps = append(steps, programStep{axis: axisSelf, any: true})
		case runtime.OpChildAny:
			steps = append(steps, programStep{axis: axisChild, any: true})
		case runtime.OpChildNSAny, runtime.OpChildName:
			steps = append(steps, programStep{axis: axisChild, op: op})
		default:
			return false
		}
	}
	return matchProgramSteps(steps, frames, startDepth, currentDepth)
}

func matchProgramSteps(steps []programStep, frames []rtIdentityFrame, startDepth, currentDepth int) bool {
	if len(steps) == 0 {
		return currentDepth == startDepth
	}
	var match func(stepIndex, nodeDepth int) bool
	match = func(stepIndex, nodeDepth int) bool {
		if nodeDepth < startDepth || nodeDepth >= len(frames) || stepIndex < 0 {
			return false
		}
		step := steps[stepIndex]
		if !rtNodeTestMatches(step, &frames[nodeDepth]) {
			return false
		}
		if stepIndex == 0 {
			return rtAxisMatchesStart(step.axis, startDepth, nodeDepth)
		}
		switch step.axis {
		case axisChild:
			return match(stepIndex-1, nodeDepth-1)
		case axisSelf:
			return match(stepIndex-1, nodeDepth)
		case axisDescendant:
			for prev := nodeDepth - 1; prev >= startDepth; prev-- {
				if match(stepIndex-1, prev) {
					return true
				}
			}
			return false
		case axisDescendantOrSelf:
			for prev := nodeDepth; prev >= startDepth; prev-- {
				if match(stepIndex-1, prev) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
	return match(len(steps)-1, currentDepth)
}

func rtAxisMatchesStart(axis stepAxis, startDepth, nodeDepth int) bool {
	switch axis {
	case axisChild:
		return nodeDepth == startDepth+1
	case axisSelf:
		return nodeDepth == startDepth
	case axisDescendant:
		return nodeDepth > startDepth
	case axisDescendantOrSelf:
		return nodeDepth >= startDepth
	default:
		return false
	}
}

func rtNodeTestMatches(step programStep, frame *rtIdentityFrame) bool {
	if frame == nil {
		return false
	}
	if step.any {
		return true
	}
	switch step.op.Op {
	case runtime.OpChildName:
		return frame.sym == step.op.Sym
	case runtime.OpChildNSAny:
		return frame.ns == step.op.NS
	default:
		return false
	}
}

func collectIdentityAttrs(rt *runtime.Schema, attrs []StartAttr, applied []AttrApplied) []rtIdentityAttr {
	if len(attrs) == 0 && len(applied) == 0 {
		return nil
	}
	out := make([]rtIdentityAttr, 0, len(attrs)+len(applied))
	for _, attr := range attrs {
		local := attr.Local
		if len(local) == 0 && attr.Sym != 0 {
			local = rt.Symbols.LocalBytes(attr.Sym)
		}
		nsBytes := attr.NSBytes
		if len(nsBytes) == 0 && attr.NS != 0 {
			nsBytes = rt.Namespaces.Bytes(attr.NS)
		}
		out = append(out, rtIdentityAttr{
			sym:      attr.Sym,
			ns:       attr.NS,
			nsBytes:  nsBytes,
			local:    local,
			keyKind:  attr.KeyKind,
			keyBytes: attr.KeyBytes,
		})
	}
	for _, ap := range applied {
		if ap.Name == 0 {
			continue
		}
		nsID := runtime.NamespaceID(0)
		if int(ap.Name) < len(rt.Symbols.NS) {
			nsID = rt.Symbols.NS[ap.Name]
		}
		out = append(out, rtIdentityAttr{
			sym:      ap.Name,
			ns:       nsID,
			nsBytes:  rt.Namespaces.Bytes(nsID),
			local:    rt.Symbols.LocalBytes(ap.Name),
			keyKind:  ap.KeyKind,
			keyBytes: ap.KeyBytes,
		})
	}
	return out
}

func isXMLNSAttr(attr *rtIdentityAttr, rt *runtime.Schema) bool {
	if rt == nil || attr == nil {
		return false
	}
	if attr.ns != 0 {
		nsBytes := rt.Namespaces.Bytes(attr.ns)
		return bytes.Equal(nsBytes, []byte(xsdxml.XMLNSNamespace))
	}
	return bytes.Equal(attr.nsBytes, []byte(xsdxml.XMLNSNamespace))
}

func attrNamespaceMatches(attr *rtIdentityAttr, ns runtime.NamespaceID, rt *runtime.Schema) bool {
	if attr == nil {
		return false
	}
	if attr.ns != 0 {
		return attr.ns == ns
	}
	if rt == nil {
		return false
	}
	return bytes.Equal(attr.nsBytes, rt.Namespaces.Bytes(ns))
}

func attrNameMatches(attr *rtIdentityAttr, op runtime.PathOp, rt *runtime.Schema) bool {
	if attr == nil {
		return false
	}
	if attr.sym != 0 {
		return attr.sym == op.Sym
	}
	if rt == nil {
		return false
	}
	targetLocal := rt.Symbols.LocalBytes(op.Sym)
	if !bytes.Equal(attr.local, targetLocal) {
		return false
	}
	return attrNamespaceMatches(attr, op.NS, rt)
}

func makeAttrKey(nsBytes, local []byte) string {
	if len(nsBytes) == 0 && len(local) == 0 {
		return ""
	}
	key := make([]byte, 0, len(nsBytes)+1+len(local))
	key = append(key, nsBytes...)
	key = append(key, 0)
	key = append(key, local...)
	return string(key)
}

func isSimpleContent(rt *runtime.Schema, typeID runtime.TypeID) bool {
	if typeID == 0 || int(typeID) >= len(rt.Types) {
		return false
	}
	typ := rt.Types[typeID]
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		return true
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(rt.ComplexTypes) {
			return false
		}
		ct := rt.ComplexTypes[typ.Complex.ID]
		return ct.Content == runtime.ContentSimple
	default:
		return false
	}
}

func elementValueKey(frame *rtIdentityFrame, elem *runtime.Element, in identityEndInput) (runtime.ValueKind, []byte, bool) {
	if elem == nil {
		return runtime.VKInvalid, nil, false
	}
	if frame.nilled {
		return runtime.VKInvalid, nil, false
	}
	if in.KeyKind == runtime.VKInvalid {
		return runtime.VKInvalid, nil, true
	}
	return in.KeyKind, in.KeyBytes, true
}

func elementByID(rt *runtime.Schema, id runtime.ElemID) (*runtime.Element, bool) {
	if rt == nil || id == 0 || int(id) >= len(rt.Elements) {
		return nil, false
	}
	return &rt.Elements[id], true
}
