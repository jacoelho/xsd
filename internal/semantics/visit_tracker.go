package semantics

import "github.com/jacoelho/xsd/internal/model"

type modelGroupVisitTracker struct {
	seen map[*model.ModelGroup]struct{}
}

func newModelGroupVisit() modelGroupVisitTracker {
	return modelGroupVisitTracker{seen: make(map[*model.ModelGroup]struct{})}
}

func (v *modelGroupVisitTracker) Enter(value *model.ModelGroup) bool {
	if v == nil {
		return false
	}
	if v.seen == nil {
		v.seen = make(map[*model.ModelGroup]struct{})
	}
	if _, ok := v.seen[value]; ok {
		return false
	}
	v.seen[value] = struct{}{}
	return true
}
