package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ResolveSelectorElementType resolves the type of the element selected by the
// selector XPath.
func ResolveSelectorElementType(schema *parser.Schema, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (model.Type, error) {
	selectorDecls, err := resolveSelectorElementDecls(schema, constraintElement, selectorXPath, nsContext)
	if err != nil {
		return nil, err
	}

	var elementType model.Type
	for _, decl := range selectorDecls {
		resolved := parser.ResolveTypeReference(schema, decl.Type)
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
	expr, err := parseXPathExpression(selectorXPath, nsContext, runtime.AttributesDisallowed)
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

// ResolveFieldElementDecl resolves a field XPath to the selected element
// declaration. It returns nil if the field selects an attribute.
func ResolveFieldElementDecl(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (*model.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, runtime.AttributesAllowed)
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

// ResolveFieldElementDecls resolves all element declarations selected by a
// field XPath.
func ResolveFieldElementDecls(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) ([]*model.ElementDecl, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, runtime.AttributesAllowed)
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

func resolveFieldElementDeclBranches(schema *parser.Schema, selectorDecls []*model.ElementDecl, fieldXPath string, paths []runtime.Path, tolerateUnresolved bool) ([]*model.ElementDecl, bool, error) {
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

// ResolveFieldType resolves the type of a field XPath expression.
func ResolveFieldType(schema *parser.Schema, field *model.Field, constraintElement *model.ElementDecl, selectorXPath string, nsContext map[string]string) (model.Type, error) {
	if field == nil {
		return nil, fmt.Errorf("field is nil")
	}
	fieldExpr, err := parseXPathExpression(field.XPath, nsContext, runtime.AttributesAllowed)
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
	resolvedTypes, unresolved, nillableFound, err := resolveFieldTypeBranches(schema, selectorDecls, field.XPath, fieldExpr.Paths)
	if err != nil {
		return nil, err
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

func resolveFieldTypeBranches(schema *parser.Schema, selectorDecls []*model.ElementDecl, fieldXPath string, paths []runtime.Path) ([]model.Type, bool, bool, error) {
	hasUnion := hasFieldPathUnion(selectorDecls, paths)
	var resolvedTypes []model.Type
	unresolved := false
	nillableFound := false
	err := forEachFieldPathBranch(selectorDecls, paths, func(branch fieldPathBranch) error {
		typ, pathErr := resolveFieldPathType(schema, branch.selectorDecl, branch.path)
		if pathErr == nil {
			resolvedTypes = append(resolvedTypes, typ)
			return nil
		}
		if errors.Is(pathErr, ErrFieldSelectsNillable) {
			if typ != nil {
				resolvedTypes = append(resolvedTypes, typ)
			}
			nillableFound = true
			if hasUnion {
				return nil
			}
			return wrapFieldPathBranchError(fieldXPath, branch, pathErr)
		}
		if hasUnion {
			unresolved = true
			return nil
		}
		return wrapFieldPathBranchError(fieldXPath, branch, pathErr)
	})
	if err != nil {
		return nil, false, false, err
	}
	return resolvedTypes, unresolved, nillableFound, nil
}
