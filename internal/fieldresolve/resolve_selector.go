package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
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
		if !model.ElementTypesCompatible(elementType, resolved) {
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
	branches := []*model.ElementDecl{constraintElement}
	err = forEachFieldPathBranch(branches, expr.Paths, func(branch fieldPathBranch) error {
		if branch.path.Attribute != nil {
			return fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
		}
		decl, resolveErr := resolvePathElementDecl(schema, branch.selectorDecl, branch.path.Steps)
		if resolveErr != nil {
			if errors.Is(resolveErr, ErrXPathUnresolvable) {
				unresolved = true
				return nil
			}
			return wrapXPathBranchError("selector", selectorXPath, branch, resolveErr)
		}
		decls = append(decls, decl)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		if unresolved {
			return nil, fmt.Errorf("%w: selector xpath '%s'", ErrXPathUnresolvable, selectorXPath)
		}
		return nil, fmt.Errorf("cannot resolve selector element")
	}
	return decls, nil
}
