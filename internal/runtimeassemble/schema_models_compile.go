package runtimeassemble

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/grouprefs"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/runtime"
	model "github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) compileParticleModel(particle model.Particle) (runtime.ModelRef, runtime.ContentKind, error) {
	if particle == nil {
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
	resolved, err := grouprefs.ExpandGroupRefs(particle, b.groupRefExpansionOptions())
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	particle = resolved
	if isEmptyChoice(particle) {
		return b.addRejectAllModel(), runtime.ContentElementOnly, nil
	}
	err = b.validateOccursLimit(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	if group, ok := particle.(*model.ModelGroup); ok && group.Kind == model.AllGroup {
		ref, addErr := b.addAllModel(group)
		if addErr != nil {
			return runtime.ModelRef{}, 0, addErr
		}
		return ref, runtime.ContentAll, nil
	}

	glu, err := models.BuildGlushkov(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	glu, err = models.ExpandSubstitution(glu, b.resolveSubstitutionHead, b.substitutionMembers)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	matchers, err := b.buildMatchers(glu)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	compiled, err := models.Compile(glu, matchers, b.limits)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	switch compiled.Kind {
	case runtime.ModelDFA:
		id := uint32(len(b.rt.Models.DFA))
		b.rt.Models.DFA = append(b.rt.Models.DFA, compiled.DFA)
		return runtime.ModelRef{Kind: runtime.ModelDFA, ID: id}, runtime.ContentElementOnly, nil
	case runtime.ModelNFA:
		id := uint32(len(b.rt.Models.NFA))
		b.rt.Models.NFA = append(b.rt.Models.NFA, compiled.NFA)
		return runtime.ModelRef{Kind: runtime.ModelNFA, ID: id}, runtime.ContentElementOnly, nil
	default:
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
}

func (b *schemaBuilder) groupRefExpansionOptions() grouprefs.ExpandGroupRefsOptions {
	return grouprefs.ExpandGroupRefsOptions{
		Lookup: func(ref *model.GroupRef) *model.ModelGroup {
			if ref == nil {
				return nil
			}
			if b != nil && b.refs != nil {
				if target, ok := b.refs.GroupRefs[ref.RefQName]; ok {
					if b != nil && b.schema != nil {
						if group := b.schema.Groups[target]; group != nil {
							return group
						}
					}
					return nil
				}
			}
			if b == nil || b.schema == nil {
				return nil
			}
			return b.schema.Groups[ref.RefQName]
		},
		MissingError: func(ref model.QName) error {
			return fmt.Errorf("group ref %s not resolved", ref)
		},
		CycleError: func(ref model.QName) error {
			return fmt.Errorf("group ref cycle detected: %s", ref)
		},
		AllGroupMode: grouprefs.AllGroupKeep,
		LeafClone:    grouprefs.LeafReuse,
	}
}

func isEmptyChoice(particle model.Particle) bool {
	group, ok := particle.(*model.ModelGroup)
	if !ok || group == nil || group.Kind != model.Choice {
		return false
	}
	for _, child := range group.Particles {
		if child == nil {
			continue
		}
		if child.MaxOcc().IsZero() {
			continue
		}
		return false
	}
	return true
}

func (b *schemaBuilder) validateOccursLimit(particle model.Particle) error {
	if particle == nil || b.maxOccurs == 0 {
		return nil
	}
	if err := b.checkOccursValue("minOccurs", particle.MinOcc()); err != nil {
		return err
	}
	if err := b.checkOccursValue("maxOccurs", particle.MaxOcc()); err != nil {
		return err
	}
	if group, ok := particle.(*model.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := b.validateOccursLimit(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *schemaBuilder) checkOccursValue(attr string, occ occurs.Occurs) error {
	if b == nil || b.maxOccurs == 0 {
		return nil
	}
	if occ.IsUnbounded() {
		return nil
	}
	if occ.IsOverflow() {
		return fmt.Errorf("%w: %s value %s exceeds uint32", occurs.ErrOccursOverflow, attr, occ.String())
	}
	if occ.GreaterThanInt(int(b.maxOccurs)) {
		return fmt.Errorf("%w: %s value %s exceeds limit %d", occurs.ErrOccursTooLarge, attr, occ.String(), b.maxOccurs)
	}
	return nil
}
