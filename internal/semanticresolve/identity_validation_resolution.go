package semanticresolve

import (
	"errors"
	"fmt"
	"strings"

	fieldresolve "github.com/jacoelho/xsd/internal/fieldresolve"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateIdentityConstraintResolution validates that identity constraint selector and fields can be resolved.
// This validation is lenient; schema-time checks avoid rejecting complex-content fields.
// Simple-typed field requirements are enforced during instance validation.
func validateIdentityConstraintResolution(sch *parser.Schema, constraint *types.IdentityConstraint, decl *types.ElementDecl) error {
	for i := range constraint.Fields {
		field := &constraint.Fields[i]
		hasUnion := strings.Contains(field.XPath, "|") || strings.Contains(constraint.Selector.XPath, "|")
		resolved, err := fieldresolve.ResolveFieldType(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
		switch {
		case err == nil:
			field.ResolvedType = resolved
		case errors.Is(err, fieldresolve.ErrFieldSelectsNillable):
			if resolved != nil {
				field.ResolvedType = resolved
			}
			if constraint.Type == types.KeyConstraint {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
			continue
		case errors.Is(err, fieldresolve.ErrFieldSelectsComplexContent):
			continue
		case hasUnion:
			if !errors.Is(err, fieldresolve.ErrXPathUnresolvable) && !errors.Is(err, fieldresolve.ErrFieldXPathIncompatibleTypes) {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
		default:
			if !errors.Is(err, fieldresolve.ErrXPathUnresolvable) && !errors.Is(err, fieldresolve.ErrFieldXPathIncompatibleTypes) {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
		}
		if constraint.Type == types.KeyConstraint {
			if hasUnion {
				elemDecls, err := fieldresolve.ResolveFieldElementDecls(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
				if err != nil {
					if errors.Is(err, fieldresolve.ErrXPathUnresolvable) {
						continue
					}
					continue
				}
				for _, elemDecl := range elemDecls {
					if elemDecl != nil && elemDecl.Nillable {
						return fmt.Errorf("field %d '%s' selects nillable element '%s'", i+1, field.XPath, elemDecl.Name)
					}
				}
				continue
			}
			elemDecl, err := fieldresolve.ResolveFieldElementDecl(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
			if err == nil && elemDecl != nil && elemDecl.Nillable {
				return fmt.Errorf("field %d '%s' selects nillable element '%s'", i+1, field.XPath, elemDecl.Name)
			}
		}
	}
	return nil
}
