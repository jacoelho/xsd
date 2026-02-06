package semanticcheck

import "github.com/jacoelho/xsd/internal/types"

type modelGroupVisit map[*types.ModelGroup]bool

func newModelGroupVisit() modelGroupVisit {
	return make(map[*types.ModelGroup]bool)
}

func (v modelGroupVisit) enter(group *types.ModelGroup) bool {
	if v[group] {
		return false
	}
	v[group] = true
	return true
}
