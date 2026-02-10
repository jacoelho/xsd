package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateElementDeclarationsConsistentInParticle validates "Element Declarations Consistent"
// across a particle tree, including nested model groups and group references.
func validateElementDeclarationsConsistentInParticle(schema *parser.Schema, particle model.Particle) error {
	seen := make(map[model.QName]model.Type)
	visited := newModelGroupVisit()
	return validateElementDeclarationsConsistentWithVisited(schema, particle, seen, visited)
}

func validateElementDeclarationsConsistentWithVisited(schema *parser.Schema, particle model.Particle, seen map[model.QName]model.Type, visited modelGroupVisit) error {
	switch p := particle.(type) {
	case *model.ModelGroup:
		if !visited.Enter(p) {
			return nil
		}
		for _, child := range p.Particles {
			if err := validateElementDeclarationsConsistentWithVisited(schema, child, seen, visited); err != nil {
				return err
			}
		}
	case *model.GroupRef:
		if schema == nil {
			return nil
		}
		if group, ok := schema.Groups[p.RefQName]; ok {
			return validateElementDeclarationsConsistentWithVisited(schema, group, seen, visited)
		}
	case *model.ElementDecl:
		elemType := p.Type
		if p.IsReference && schema != nil {
			if refDecl, ok := schema.ElementDecls[p.Name]; ok {
				elemType = refDecl.Type
			}
		}
		if elemType == nil {
			return nil
		}
		if existing, ok := seen[p.Name]; ok {
			if !ElementTypesCompatible(existing, elemType) {
				return fmt.Errorf("element declarations consistent violation for element '%s'", p.Name)
			}
			return nil
		}
		seen[p.Name] = elemType
	}
	return nil
}
