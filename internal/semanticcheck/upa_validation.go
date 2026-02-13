package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/grouprefs"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
)

// ValidateUPA validates Unique Particle Attribution for a content model.
// UPA requires that no element can be matched by more than one particle.
func ValidateUPA(schema *parser.Schema, content model.Content, _ model.NamespaceURI) error {
	particle, baseParticle := upaParticles(schema, content)
	if particle == nil && baseParticle == nil {
		return nil
	}

	expandOptions := grouprefs.ExpandGroupRefsOptions{
		Lookup: func(ref *model.GroupRef) *model.ModelGroup {
			if schema == nil || ref == nil {
				return nil
			}
			return schema.Groups[ref.RefQName]
		},
		MissingError: func(ref model.QName) error {
			if schema == nil {
				return fmt.Errorf("group ref %s not resolved", ref)
			}
			return fmt.Errorf("group '%s' not found", ref)
		},
		CycleError: func(ref model.QName) error {
			return fmt.Errorf("circular group reference detected for %s", ref)
		},
		AllGroupMode: grouprefs.AllGroupAsChoice,
		LeafClone:    grouprefs.LeafClone,
	}

	var err error
	particle, err = expandAndRelaxParticle(particle, expandOptions)
	if err != nil {
		return err
	}
	baseParticle, err = expandAndRelaxParticle(baseParticle, expandOptions)
	if err != nil {
		return err
	}

	particle = combineBaseAndDerivedUPAParticles(baseParticle, particle)
	if particle == nil {
		return nil
	}

	glu, err := contentmodel.BuildGlushkov(particle)
	if err != nil {
		return err
	}
	checker := newUPAChecker(schema)
	return contentmodel.CheckDeterminism(glu, checker.positionsOverlap)
}

func upaParticles(schema *parser.Schema, content model.Content) (model.Particle, model.Particle) {
	var particle model.Particle
	var baseParticle model.Particle

	switch c := content.(type) {
	case *model.ElementContent:
		particle = c.Particle
	case *model.ComplexContent:
		if c.Extension != nil {
			particle = c.Extension.Particle
			if !c.Extension.Base.IsZero() {
				if baseCT, ok := typechain.LookupComplexType(schema, c.Extension.Base); ok {
					if baseEC, ok := baseCT.Content().(*model.ElementContent); ok {
						baseParticle = baseEC.Particle
					}
				}
			}
		}
		if c.Restriction != nil {
			particle = c.Restriction.Particle
		}
	}

	return particle, baseParticle
}

func expandAndRelaxParticle(particle model.Particle, opts grouprefs.ExpandGroupRefsOptions) (model.Particle, error) {
	if particle == nil {
		return nil, nil
	}
	expanded, err := grouprefs.ExpandGroupRefs(particle, opts)
	if err != nil {
		return nil, err
	}
	return relaxOccursCopy(expanded), nil
}

func combineBaseAndDerivedUPAParticles(baseParticle, particle model.Particle) model.Particle {
	if baseParticle != nil && particle != nil {
		return &model.ModelGroup{
			Kind:      model.Sequence,
			MinOccurs: occurs.OccursFromInt(1),
			MaxOccurs: occurs.OccursFromInt(1),
			Particles: []model.Particle{baseParticle, particle},
		}
	}
	if particle == nil {
		return baseParticle
	}
	return particle
}
