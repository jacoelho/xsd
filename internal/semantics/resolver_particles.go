package semantics

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
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
			if err := r.resolveGroupRefParticle(particle); err != nil {
				return err
			}
		case *model.ModelGroup:
			queue = append(queue, particle.Particles...)
		case *model.ElementDecl:
			if err := r.resolveElementDeclParticle(particle); err != nil {
				return err
			}
		case *model.AnyElement:
			// wildcards don't need resolution
		}
	}
	return nil
}

func (r *Resolver) resolveGroupRefParticle(ref *model.GroupRef) error {
	group, ok := r.schema.Groups[ref.RefQName]
	if !ok {
		return fmt.Errorf("group %s not found", ref.RefQName)
	}
	return analysis.ResolveNamed[model.QName](r.detector, ref.RefQName, func() error {
		return r.resolveParticles(group.Particles)
	})
}

func (r *Resolver) resolveElementDeclParticle(elem *model.ElementDecl) error {
	if elem.IsReference || elem.Type == nil {
		return nil
	}
	return r.resolveElementType(elem, elem.Name, elementTypeOptions{
		simpleContext:  "element %s type: %w",
		complexContext: "element %s anonymous type: %w",
		allowResolving: true,
	})
}

func (r *Resolver) resolveContentParticles(content model.Content) error {
	return model.WalkContentParticles(content, func(particle model.Particle) error {
		return r.resolveParticles([]model.Particle{particle})
	})
}
