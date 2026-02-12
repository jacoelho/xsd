package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
