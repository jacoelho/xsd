package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) compileParticleModel(id schemair.ParticleID) (runtime.ModelRef, runtime.ContentKind, error) {
	if id == 0 {
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
	particle, err := b.particleTree(id)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	return b.compileParticleTree(particle)
}

func (b *schemaBuilder) compileParticleTree(particle contentmodel.TreeParticle) (runtime.ModelRef, runtime.ContentKind, error) {
	if isEmptyChoice(particle) {
		ref, err := b.addRejectAllModel()
		return ref, runtime.ContentElementOnly, err
	}
	if err := b.validateOccursLimit(particle); err != nil {
		return runtime.ModelRef{}, 0, err
	}
	if particle.Kind == contentmodel.TreeGroup && particle.Group == contentmodel.TreeAll {
		ref, addErr := b.addAllModel(particle)
		if addErr != nil {
			return runtime.ModelRef{}, 0, addErr
		}
		return ref, runtime.ContentAll, nil
	}

	glu, err := contentmodel.BuildGlushkovTree(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	glu, err = contentmodel.ExpandSubstitutionIDs(glu, b.substitutionMembers)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	matchers, err := b.buildMatchers(glu)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	compiled, err := contentmodel.CompileContentModel(glu, matchers, b.limits)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	switch compiled.Kind {
	case runtime.ModelDFA:
		ref, err := b.assembler.AppendDFAModel(compiled.DFA)
		return ref, runtime.ContentElementOnly, err
	case runtime.ModelNFA:
		ref, err := b.assembler.AppendNFAModel(compiled.NFA)
		return ref, runtime.ContentElementOnly, err
	default:
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
}

func isEmptyChoice(particle contentmodel.TreeParticle) bool {
	if particle.Kind != contentmodel.TreeGroup || particle.Group != contentmodel.TreeChoice {
		return false
	}
	for _, child := range particle.Children {
		if treeOccursZero(child.Max) {
			continue
		}
		return false
	}
	return true
}

func (b *schemaBuilder) validateOccursLimit(particle contentmodel.TreeParticle) error {
	if b.maxOccurs == 0 {
		return nil
	}
	if err := b.checkOccursValue("minOccurs", particle.Min); err != nil {
		return err
	}
	if err := b.checkOccursValue("maxOccurs", particle.Max); err != nil {
		return err
	}
	if particle.Kind == contentmodel.TreeGroup {
		for _, child := range particle.Children {
			if err := b.validateOccursLimit(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *schemaBuilder) checkOccursValue(attr string, occ contentmodel.TreeOccurs) error {
	if b == nil || b.maxOccurs == 0 {
		return nil
	}
	if occ.Unbounded {
		return nil
	}
	if occ.Value > b.maxOccurs {
		return fmt.Errorf("SCHEMA_OCCURS_TOO_LARGE: %s value %d exceeds limit %d", attr, occ.Value, b.maxOccurs)
	}
	return nil
}

func (b *schemaBuilder) particleTree(id schemair.ParticleID) (contentmodel.TreeParticle, error) {
	if id == 0 || int(id) > len(b.schema.Particles) {
		return contentmodel.TreeParticle{}, fmt.Errorf("runtime build: particle %d out of range", id)
	}
	particle := b.schema.Particles[id-1]
	minOccurs, maxOccurs := particle.OccursRange()
	out := contentmodel.TreeParticle{
		ElementID:          uint32(particle.ElementRef()),
		WildcardID:         uint32(particle.WildcardRef()),
		Min:                treeOccurs(minOccurs),
		Max:                treeOccurs(maxOccurs),
		AllowsSubstitution: particle.AllowsSubstitutionGroup(),
	}
	switch kind := particle.ParticleKind(); kind {
	case schemair.ParticleElement:
		out.Kind = contentmodel.TreeElement
	case schemair.ParticleWildcard:
		out.Kind = contentmodel.TreeWildcard
	case schemair.ParticleGroup:
		out.Kind = contentmodel.TreeGroup
		out.Group = treeGroup(particle.GroupKind())
		for _, child := range particle.ChildParticles() {
			childTree, err := b.particleTree(child)
			if err != nil {
				return contentmodel.TreeParticle{}, err
			}
			out.Children = append(out.Children, childTree)
		}
	default:
		return contentmodel.TreeParticle{}, fmt.Errorf("runtime build: unsupported particle kind %d", kind)
	}
	return out, nil
}

func treeOccurs(value schemair.Occurs) contentmodel.TreeOccurs {
	return contentmodel.TreeOccurs{Value: value.Value, Unbounded: value.Unbounded}
}

func treeOccursZero(value contentmodel.TreeOccurs) bool {
	return !value.Unbounded && value.Value == 0
}

func treeGroup(value schemair.GroupKind) contentmodel.TreeGroupKind {
	switch value {
	case schemair.GroupChoice:
		return contentmodel.TreeChoice
	case schemair.GroupAll:
		return contentmodel.TreeAll
	default:
		return contentmodel.TreeSequence
	}
}
