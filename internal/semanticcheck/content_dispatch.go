package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type typeDefinitionContext int

const (
	typeDefinitionGlobal typeDefinitionContext = iota
	typeDefinitionInline
)

// validateContentStructure validates structural constraints of content
// Does not validate references (which might be forward references or imports)
// context indicates if this content is part of an inline complexType (local element)
func validateContentStructure(schema *parser.Schema, content model.Content, context typeDefinitionContext) error {
	switch c := content.(type) {
	case *model.ElementContent:
		if err := validateParticleStructure(schema, c.Particle); err != nil {
			return err
		}
		if err := validateElementDeclarationsConsistentInParticle(schema, c.Particle); err != nil {
			return err
		}
	case *model.SimpleContent:
		return validateSimpleContentStructure(schema, c, context)
	case *model.ComplexContent:
		return validateComplexContentStructure(schema, c)
	case *model.EmptyContent:
	}
	return nil
}
