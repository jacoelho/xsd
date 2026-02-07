package semanticresolve

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveGroup(qname types.QName, mg *types.ModelGroup) error {
	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
		// resolve group particles (expand GroupRefs)
		return r.resolveParticles(mg.Particles)
	})
}

func (r *Resolver) resolveParticles(particles []types.Particle) error {
	// use iterative approach with work queue to avoid stack overflow
	// inline ModelGroups are tree-structured (no pointer cycles)
	// named groups (GroupRef) have cycle detection via r.detector
	queue := slices.Clone(particles)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]

		switch particle := p.(type) {
		case *types.GroupRef:
			if err := r.resolveGroupRefParticle(particle); err != nil {
				return err
			}
		case *types.ModelGroup:
			queue = append(queue, particle.Particles...)
		case *types.ElementDecl:
			if err := r.resolveElementParticle(particle); err != nil {
				return err
			}
		case *types.AnyElement:
			// wildcards don't need resolution
		}
	}
	return nil
}

func (r *Resolver) resolveGroupRefParticle(ref *types.GroupRef) error {
	group, ok := r.schema.Groups[ref.RefQName]
	if !ok {
		return fmt.Errorf("group %s not found", ref.RefQName)
	}
	return r.resolveGroup(ref.RefQName, group)
}

func (r *Resolver) resolveContentParticles(content types.Content) error {
	return traversal.WalkContentParticles(content, func(particle types.Particle) error {
		return r.resolveParticles([]types.Particle{particle})
	})
}
