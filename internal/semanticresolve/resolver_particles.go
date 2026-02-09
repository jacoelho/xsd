package semanticresolve

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

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
			group, ok := r.schema.Groups[particle.RefQName]
			if !ok {
				return fmt.Errorf("group %s not found", particle.RefQName)
			}
			if r.detector.IsVisited(particle.RefQName) {
				continue
			}
			if err := r.detector.WithScope(particle.RefQName, func() error {
				return r.resolveParticles(group.Particles)
			}); err != nil {
				return err
			}
		case *types.ModelGroup:
			queue = append(queue, particle.Particles...)
		case *types.ElementDecl:
			if particle.IsReference || particle.Type == nil {
				continue
			}
			if err := r.resolveElementType(particle, particle.Name, elementTypeOptions{
				simpleContext:  "element %s type: %w",
				complexContext: "element %s anonymous type: %w",
				allowResolving: true,
			}); err != nil {
				return err
			}
		case *types.AnyElement:
			// wildcards don't need resolution
		}
	}
	return nil
}

func (r *Resolver) resolveContentParticles(content types.Content) error {
	return traversal.WalkContentParticles(content, func(particle types.Particle) error {
		return r.resolveParticles([]types.Particle{particle})
	})
}
