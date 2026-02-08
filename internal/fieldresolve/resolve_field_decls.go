package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// ResolveFieldElementDecl resolves a field XPath to the selected element declaration.
// Returns nil if the field selects an attribute.
func ResolveFieldElementDecl(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (*types.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}

	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}

	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		if errors.Is(err, ErrFieldXPathIncompatibleTypes) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: resolve selector '%s': %w", ErrFieldXPathUnresolved, selectorXPath, err)
	}

	var decls []*types.ElementDecl
	for _, selectorDecl := range selectorDecls {
		for pathIndex, fieldPath := range fieldExpr.Paths {
			if fieldPath.Attribute != nil {
				return nil, fmt.Errorf("field xpath selects attribute: %s", field.XPath)
			}
			decl, err := resolvePathElementDecl(schema, selectorDecl, fieldPath.Steps)
			if err != nil {
				return nil, fmt.Errorf("resolve field xpath '%s' branch %d: %w", field.XPath, pathIndex+1, err)
			}
			decls = append(decls, decl)
		}
	}

	unique := uniqueElementDecls(decls)
	if len(unique) != 1 {
		return nil, fmt.Errorf("field xpath '%s' resolves to multiple element declarations", field.XPath)
	}
	return unique[0], nil
}

func ResolveFieldElementDecls(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*types.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}
	hasUnion := len(fieldExpr.Paths) > 1 || len(selectorDecls) > 1
	var decls []*types.ElementDecl
	unresolved := false
	for _, selectorDecl := range selectorDecls {
		for pathIndex, fieldPath := range fieldExpr.Paths {
			if fieldPath.Attribute != nil {
				continue
			}
			decl, err := resolvePathElementDecl(schema, selectorDecl, fieldPath.Steps)
			if err != nil {
				if hasUnion {
					unresolved = true
					continue
				}
				if errors.Is(err, ErrXPathUnresolvable) {
					unresolved = true
					continue
				}
				return nil, fmt.Errorf("resolve field xpath '%s' branch %d: %w", field.XPath, pathIndex+1, err)
			}
			decls = append(decls, decl)
		}
	}
	unique := uniqueElementDecls(decls)
	if len(unique) == 0 && unresolved {
		return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
	}
	return unique, nil
}
