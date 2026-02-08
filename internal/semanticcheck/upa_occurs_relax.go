package semanticcheck

import "github.com/jacoelho/xsd/internal/types"

func relaxOccursCopy(particle types.Particle) types.Particle {
	switch typed := particle.(type) {
	case *types.ElementDecl:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		return &clone
	case *types.AnyElement:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		return &clone
	case *types.ModelGroup:
		if typed == nil {
			return nil
		}
		clone := *typed
		clone.MinOccurs, clone.MaxOccurs = relaxOccurs(typed.MinOccurs, typed.MaxOccurs)
		if len(typed.Particles) > 0 {
			clone.Particles = make([]types.Particle, 0, len(typed.Particles))
			for _, child := range typed.Particles {
				clone.Particles = append(clone.Particles, relaxOccursCopy(child))
			}
		}
		return &clone
	default:
		return particle
	}
}

func relaxOccurs(minOccurs, maxOccurs types.Occurs) (types.Occurs, types.Occurs) {
	if maxOccurs.IsUnbounded() || maxOccurs.GreaterThanInt(1) {
		if minOccurs.IsZero() {
			return types.OccursFromInt(0), types.OccursUnbounded
		}
		return types.OccursFromInt(1), types.OccursUnbounded
	}
	return minOccurs, maxOccurs
}
