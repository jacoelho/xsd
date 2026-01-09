package validation

import (
	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateContentStructure validates structural constraints of content
// Does not validate references (which might be forward references or imports)
// isInline indicates if this content is part of an inline complexType (local element)
func validateContentStructure(schema *schema.Schema, content types.Content, isInline bool) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if err := validateParticleStructure(schema, c.Particle, nil); err != nil {
			return err
		}
		if err := validateElementDeclarationsConsistentInParticle(schema, c.Particle); err != nil {
			return err
		}
	case *types.SimpleContent:
		return validateSimpleContentStructure(schema, c, isInline)
	case *types.ComplexContent:
		return validateComplexContentStructure(schema, c)
	case *types.EmptyContent:
		// empty content is always valid
	}
	return nil
}
