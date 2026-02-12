package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/traversal"
)

func newModelGroupVisit() traversal.VisitTracker[*model.ModelGroup] {
	return traversal.NewVisitTracker[*model.ModelGroup]()
}
