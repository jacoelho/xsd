package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/traversal"
)

type modelGroupVisit = traversal.VisitTracker[*model.ModelGroup]

func newModelGroupVisit() modelGroupVisit {
	return traversal.NewVisitTracker[*model.ModelGroup]()
}
