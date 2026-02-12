package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
