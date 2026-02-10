package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/xpath"
)

// ResolveSelectorElementType resolves the type of the element selected by the selector XPath.
func ResolveSelectorElementType(schema *parser.Schema, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (model.Type, error) {
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, err
	}

	var elementType model.Type
	for _, decl := range selectorDecls {
		resolved := typeresolve.ResolveTypeReference(schema, decl.Type, typeresolve.TypeReferenceMustExist)
		if resolved == nil {
			return nil, fmt.Errorf("cannot resolve constraint element type")
		}
		if elementType == nil {
			elementType = resolved
			continue
		}
		if !elementTypesCompatible(elementType, resolved) {
			return nil, fmt.Errorf("selector xpath '%s' resolves to multiple element types", selectorXPath)
		}
	}
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve constraint element type")
	}
	return elementType, nil
}

func resolveSelectorElementDecls(schema *parser.Schema, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*model.ElementDecl, error) {
	if constraintElement == nil {
		return nil, fmt.Errorf("constraint element is nil")
	}
	expr, err := parseXPathExpression(selectorXPath, nsContext, xpath.AttributesDisallowed)
	if err != nil {
		return nil, err
	}
	decls := make([]*model.ElementDecl, 0, len(expr.Paths))
	unresolved := false
	for i, path := range expr.Paths {
		if path.Attribute != nil {
			return nil, fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
		}
		decl, err := resolvePathElementDecl(schema, constraintElement, path.Steps)
		if err != nil {
			if errors.Is(err, ErrXPathUnresolvable) {
				unresolved = true
				continue
			}
			return nil, fmt.Errorf("resolve selector xpath '%s' branch %d: %w", selectorXPath, i+1, err)
		}
		decls = append(decls, decl)
	}
	if len(decls) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: selector xpath '%s'", ErrXPathUnresolvable, selectorXPath)
		}
		return nil, fmt.Errorf("cannot resolve selector element")
	}
	return decls, nil
}
