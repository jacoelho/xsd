package validator

import "github.com/jacoelho/xsd/internal/runtime"

// Frame is the minimal element-path view needed by identity path matching.
type Frame interface {
	MatchSymbol() runtime.SymbolID
	MatchNamespace() runtime.NamespaceID
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

// MatchesAnySelector reports whether any selector path matches the current frame.
func MatchesAnySelector[F Frame](rt *runtime.Schema, selectors []runtime.PathID, frames []F, startDepth, currentDepth int) bool {
	for _, pathID := range selectors {
		ops, ok := PathOps(rt, pathID)
		if !ok {
			continue
		}
		if MatchProgramPath(ops, frames, startDepth, currentDepth) {
			return true
		}
	}
	return false
}

// PathOps returns the compiled operations for one path program.
func PathOps(rt *runtime.Schema, id runtime.PathID) ([]runtime.PathOp, bool) {
	path, ok := rt.Path(id)
	if !ok {
		return nil, false
	}
	return path.Ops, true
}

// SplitAttrOp removes a trailing attribute step from a path program.
func SplitAttrOp(ops []runtime.PathOp) ([]runtime.PathOp, runtime.PathOp, bool) {
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

// MatchProgramPath reports whether the compiled path matches the current frame.
func MatchProgramPath[F Frame](ops []runtime.PathOp, frames []F, startDepth, currentDepth int) bool {
	if currentDepth < startDepth || currentDepth >= len(frames) {
		return false
	}
	if len(ops) == 0 {
		return currentDepth == startDepth
	}
	steps, ok := compileProgramSteps(ops)
	if !ok {
		return false
	}
	return matchProgramSteps(steps, frames, startDepth, currentDepth)
}

func compileProgramSteps(ops []runtime.PathOp) ([]programStep, bool) {
	if len(ops) == 0 {
		return nil, true
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
			return nil, false
		}
	}
	return steps, true
}

func matchProgramSteps[F Frame](steps []programStep, frames []F, startDepth, currentDepth int) bool {
	if len(steps) == 0 {
		return currentDepth == startDepth
	}
	var match func(stepIndex, nodeDepth int) bool
	match = func(stepIndex, nodeDepth int) bool {
		if nodeDepth < startDepth || nodeDepth >= len(frames) || stepIndex < 0 {
			return false
		}
		step := steps[stepIndex]
		if !nodeTestMatches(step, frames[nodeDepth]) {
			return false
		}
		if stepIndex == 0 {
			return axisMatchesStart(step.axis, startDepth, nodeDepth)
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

func axisMatchesStart(axis stepAxis, startDepth, nodeDepth int) bool {
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

func nodeTestMatches[F Frame](step programStep, frame F) bool {
	if step.any {
		return true
	}
	switch step.op.Op {
	case runtime.OpChildName:
		return frame.MatchSymbol() == step.op.Sym
	case runtime.OpChildNSAny:
		return frame.MatchNamespace() == step.op.NS
	default:
		return false
	}
}
