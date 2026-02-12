package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
