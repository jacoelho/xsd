package loader

import "github.com/jacoelho/xsd/internal/types"

func normalizePointlessParticle(p types.Particle) types.Particle {
	for {
		mg, ok := p.(*types.ModelGroup)
		if !ok || mg == nil {
			return p
		}
		if mg.MinOccurs != 1 || mg.MaxOccurs != 1 {
			return p
		}
		children := derivationChildren(mg)
		if len(children) != 1 {
			return p
		}
		p = children[0]
	}
}

func derivationChildren(mg *types.ModelGroup) []types.Particle {
	if mg == nil {
		return nil
	}
	children := make([]types.Particle, 0, len(mg.Particles))
	for _, child := range mg.Particles {
		children = append(children, gatherPointlessChildren(mg.Kind, child)...)
	}
	return children
}

func gatherPointlessChildren(parentKind types.GroupKind, particle types.Particle) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if p.MinOccurs != 1 || p.MaxOccurs != 1 {
			return []types.Particle{p}
		}
		if len(p.Particles) == 1 {
			return gatherPointlessChildren(parentKind, p.Particles[0])
		}
		if p.Kind == parentKind {
			var out []types.Particle
			for _, child := range p.Particles {
				out = append(out, gatherPointlessChildren(parentKind, child)...)
			}
			return out
		}
		return []types.Particle{p}
	default:
		return []types.Particle{p}
	}
}
