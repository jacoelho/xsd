package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
)

func relaxOccursCopy(particle model.Particle) model.Particle {
	switch typed := particle.(type) {
	case *model.ElementDecl:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		return &clone
	case *model.AnyElement:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		return &clone
	case *model.ModelGroup:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		if len(typed.Particles) > 0 {
			clone.Particles = make([]model.Particle, 0, len(typed.Particles))
			for _, child := range typed.Particles {
				clone.Particles = append(clone.Particles, relaxOccursCopy(child))
			}
		}
		return &clone
	default:
		return particle
	}
}

func relaxOccurs(minOccurs, maxOccurs occurs.Occurs) (occurs.Occurs, occurs.Occurs) {
	if maxOccurs.IsUnbounded() || maxOccurs.GreaterThanInt(1) {
		if minOccurs.IsZero() {
			return occurs.OccursFromInt(0), occurs.OccursUnbounded
		}
		return occurs.OccursFromInt(1), occurs.OccursUnbounded
	}
	return minOccurs, maxOccurs
}
