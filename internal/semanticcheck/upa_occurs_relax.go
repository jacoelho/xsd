package semanticcheck

import "github.com/jacoelho/xsd/internal/model"

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

func relaxOccurs(minOccurs, maxOccurs model.Occurs) (model.Occurs, model.Occurs) {
	if maxOccurs.IsUnbounded() || maxOccurs.GreaterThanInt(1) {
		if minOccurs.IsZero() {
			return model.OccursFromInt(0), model.OccursUnbounded
		}
		return model.OccursFromInt(1), model.OccursUnbounded
	}
	return minOccurs, maxOccurs
}
