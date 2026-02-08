package semanticcheck

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemaops"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateUPA validates Unique Particle Attribution for a content model.
// UPA requires that no element can be matched by more than one particle.
func ValidateUPA(schema *parser.Schema, content types.Content, _ types.NamespaceURI) error {
	particle, baseParticle := upaParticles(schema, content)
	if particle == nil && baseParticle == nil {
		return nil
	}

	expandOptions := schemaops.ExpandGroupRefsOptions{
		Lookup: func(ref *types.GroupRef) *types.ModelGroup {
			if schema == nil || ref == nil {
				return nil
			}
			return schema.Groups[ref.RefQName]
		},
		MissingError: func(ref types.QName) error {
			if schema == nil {
				return fmt.Errorf("group ref %s not resolved", ref)
			}
			return fmt.Errorf("group '%s' not found", ref)
		},
		CycleError: func(ref types.QName) error {
			return fmt.Errorf("circular group reference detected for %s", ref)
		},
		AllGroupMode: schemaops.AllGroupAsChoice,
		LeafClone:    schemaops.LeafClone,
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

	glu, err := models.BuildGlushkov(particle)
	if err != nil {
		return err
	}
	checker := newUPAChecker(schema)
	return models.CheckDeterminism(glu, checker.positionsOverlap)
}

func upaParticles(schema *parser.Schema, content types.Content) (types.Particle, types.Particle) {
	var particle types.Particle
	var baseParticle types.Particle

	switch c := content.(type) {
	case *types.ElementContent:
		particle = c.Particle
	case *types.ComplexContent:
		if c.Extension != nil {
			particle = c.Extension.Particle
			if !c.Extension.Base.IsZero() {
				if baseCT, ok := lookupComplexType(schema, c.Extension.Base); ok {
					if baseEC, ok := baseCT.Content().(*types.ElementContent); ok {
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

func expandAndRelaxParticle(particle types.Particle, opts schemaops.ExpandGroupRefsOptions) (types.Particle, error) {
	if particle == nil {
		return nil, nil
	}
	expanded, err := schemaops.ExpandGroupRefs(particle, opts)
	if err != nil {
		return nil, err
	}
	return relaxOccursCopy(expanded), nil
}

func combineBaseAndDerivedUPAParticles(baseParticle, particle types.Particle) types.Particle {
	if baseParticle != nil && particle != nil {
		return &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
			Particles: []types.Particle{baseParticle, particle},
		}
	}
	if particle == nil {
		return baseParticle
	}
	return particle
}
