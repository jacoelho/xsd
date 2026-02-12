package fieldresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xpath"
)

// ResolveFieldElementDecl resolves a field XPath to the selected element declaration.
// Returns nil if the field selects an attribute.
func ResolveFieldElementDecl(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (*model.ElementDecl, error) {
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

	decls, _, err := resolveFieldElementDeclBranches(schema, selectorDecls, field.XPath, fieldExpr.Paths, false)
	if err != nil {
		return nil, err
	}

	unique := uniqueElementDecls(decls)
	if len(unique) != 1 {
		return nil, fmt.Errorf("field xpath '%s' resolves to multiple element declarations", field.XPath)
	}
	return unique[0], nil
}

// ResolveFieldElementDecls resolves all element declarations selected by a field XPath.
func ResolveFieldElementDecls(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*model.ElementDecl, error) {
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
	decls, unresolved, err := resolveFieldElementDeclBranches(schema, selectorDecls, field.XPath, fieldExpr.Paths, true)
	if err != nil {
		return nil, err
	}
	unique := uniqueElementDecls(decls)
	if len(unique) == 0 && unresolved {
		return nil, fmt.Errorf("%w: field xpath '%s'", ErrXPathUnresolvable, field.XPath)
	}
	return unique, nil
}

func resolveFieldElementDeclBranches(
	schema *parser.Schema,
	selectorDecls []*model.ElementDecl,
	fieldXPath string,
	paths []xpath.Path,
	tolerateUnresolved bool,
) ([]*model.ElementDecl, bool, error) {
	hasUnion := hasFieldPathUnion(selectorDecls, paths)
	var decls []*model.ElementDecl
	unresolved := false
	err := forEachFieldPathBranch(selectorDecls, paths, func(branch fieldPathBranch) error {
		if branch.path.Attribute != nil {
			if tolerateUnresolved {
				return nil
			}
			return fmt.Errorf("field xpath selects attribute: %s", fieldXPath)
		}
		decl, err := resolvePathElementDecl(schema, branch.selectorDecl, branch.path.Steps)
		if err != nil {
			if tolerateUnresolved && (hasUnion || errors.Is(err, ErrXPathUnresolvable)) {
				unresolved = true
				return nil
			}
			return wrapFieldPathBranchError(fieldXPath, branch, err)
		}
		decls = append(decls, decl)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return decls, unresolved, nil
}
