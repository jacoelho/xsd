package validator

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

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
	for _, op := range ops {
		switch op.Op {
		case runtime.OpDescend:
			steps = append(steps, programStep{axis: axisDescendantOrSelf, any: true})
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
