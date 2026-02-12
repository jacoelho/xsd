package semanticresolve

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/resolveguard"
	"github.com/jacoelho/xsd/internal/traversal"
	model "github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveParticles(particles []model.Particle) error {
	// use iterative approach with work queue to avoid stack overflow
	// inline ModelGroups are tree-structured (no pointer cycles)
	// named groups (GroupRef) have cycle detection via r.detector
	queue := slices.Clone(particles)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]

		switch particle := p.(type) {
		case *model.GroupRef:
			group, ok := r.schema.Groups[particle.RefQName]
			if !ok {
				return fmt.Errorf("group %s not found", particle.RefQName)
			}
			if err := resolveguard.ResolveNamed[model.QName](r.detector, particle.RefQName, func() error {
				return r.resolveParticles(group.Particles)
			}); err != nil {
				return err
			}
		case *model.ModelGroup:
			queue = append(queue, particle.Particles...)
		case *model.ElementDecl:
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
		case *model.AnyElement:
			// wildcards don't need resolution
		}
	}
	return nil
}

func (r *Resolver) resolveContentParticles(content model.Content) error {
	return traversal.WalkContentParticles(content, func(particle model.Particle) error {
		return r.resolveParticles([]model.Particle{particle})
	})
}
