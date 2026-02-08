package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
