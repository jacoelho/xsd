package validator

import "github.com/jacoelho/xsd/internal/runtime"

func matchProgramPath(ops []runtime.PathOp, frames []rtIdentityFrame, startDepth, currentDepth int) bool {
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
