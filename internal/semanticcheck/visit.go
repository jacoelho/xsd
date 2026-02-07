package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

type modelGroupVisit = traversal.VisitTracker[*types.ModelGroup]

func newModelGroupVisit() modelGroupVisit {
	return traversal.NewVisitTracker[*types.ModelGroup]()
}
