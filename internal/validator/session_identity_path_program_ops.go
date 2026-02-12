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
