package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// ResolveFieldType resolves the type of a field XPath expression.
// Returns the type of the attribute or element selected by the field.
// The selectorXPath is used to determine the context element (the element selected by the selector).
func ResolveFieldType(schema *parser.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string, nsContext map[string]string) (types.Type, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}

	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, xpath.AttributesAllowed)
	if err != nil {
		return nil, err
	}

	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		if errors.Is(err, ErrXPathUnresolvable) {
			return nil, err
		}
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}

	fieldHasUnion := len(fieldExpr.Paths) > 1
	selectorHasUnion := len(selectorDecls) > 1
	hasUnion := fieldHasUnion || selectorHasUnion

	var resolvedTypes []types.Type
	unresolved := false
	nillableFound := false
	for _, selectorDecl := range selectorDecls {
		for _, fieldPath := range fieldExpr.Paths {
			typ, pathErr := resolveFieldPathType(schema, selectorDecl, fieldPath)
			if pathErr != nil {
				if errors.Is(pathErr, ErrFieldSelectsNillable) {
					if typ != nil {
						resolvedTypes = append(resolvedTypes, typ)
					}
					nillableFound = true
					if hasUnion {
						continue
					}
					return nil, fmt.Errorf("resolve field xpath '%s': %w", field.XPath, pathErr)
				}
				if hasUnion {
					unresolved = true
					continue
				}
				return nil, fmt.Errorf("resolve field xpath '%s': %w", field.XPath, pathErr)
			}
			resolvedTypes = append(resolvedTypes, typ)
		}
	}

	if len(resolvedTypes) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
		}
		return nil, fmt.Errorf("field xpath '%s' resolves to no types", field.XPath)
	}
	combined, err := combineFieldTypes(field.XPath, resolvedTypes)
	if err != nil {
		if selectorHasUnion && !fieldHasUnion && errors.Is(err, ErrFieldXPathIncompatibleTypes) {
			return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
		}
		return nil, err
	}
	if nillableFound {
		return combined, fmt.Errorf("%w: field xpath '%s'", ErrFieldSelectsNillable, field.XPath)
	}
	if unresolved {
		return combined, fmt.Errorf("%w: field xpath '%s' contains wildcard branches", ErrXPathUnresolvable, field.XPath)
	}
	return combined, nil
}
