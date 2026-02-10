package fieldresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xpath"
)

func parseXPathExpression(expr string, nsContext map[string]string, policy xpath.AttributePolicy) (xpath.Expression, error) {
	parsed, err := xpath.Parse(expr, nsContext, policy)
	if err != nil {
		return xpath.Expression{}, err
	}
	if len(parsed.Paths) == 0 {
		return xpath.Expression{}, fmt.Errorf("xpath contains no paths")
	}
	return parsed, nil
}

func isWildcardNodeTest(test xpath.NodeTest) bool {
	return test.Any || test.Local == "*"
}

func nodeTestMatchesQName(test xpath.NodeTest, name model.QName) bool {
	test = xpath.CanonicalizeNodeTest(test)
	if test.Any {
		return true
	}
	if test.Local != "*" && name.Local != test.Local {
		return false
	}
	if test.NamespaceSpecified && name.Namespace != test.Namespace {
		return false
	}
	return true
}

func resolveElementReference(schema *parser.Schema, decl *model.ElementDecl) *model.ElementDecl {
	if decl == nil || !decl.IsReference || schema == nil {
		return decl
	}
	if resolved, ok := schema.ElementDecls[decl.Name]; ok {
		return resolved
	}
	return decl
}

func formatNodeTest(test xpath.NodeTest) string {
	if isWildcardNodeTest(test) {
		return "*"
	}
	if !test.NamespaceSpecified || test.Namespace == "" {
		return test.Local
	}
	return "{" + test.Namespace + "}" + test.Local
}

func fieldTypeName(typ model.Type) string {
	if typ == nil {
		return "<nil>"
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T", typ)
}

func fieldTypeKey(typ model.Type) string {
	if typ == nil {
		return ""
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T:%p", typ, typ)
}

func uniqueFieldTypes(values []model.Type) []model.Type {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]model.Type, 0, len(values))
	for _, typ := range values {
		if typ == nil {
			continue
		}
		key := fieldTypeKey(typ)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, typ)
	}
	return unique
}

func fieldTypesCompatible(a, b model.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name() == b.Name() {
		return true
	}
	if model.IsDerivedFrom(a, b) || model.IsDerivedFrom(b, a) {
		return true
	}
	primA := a.PrimitiveType()
	primB := b.PrimitiveType()
	if primA != nil && primB != nil && primA.Name() == primB.Name() {
		return true
	}
	return false
}

func combineFieldTypes(fieldXPath string, values []model.Type) (model.Type, error) {
	unique := uniqueFieldTypes(values)
	if len(unique) == 0 {
		return nil, fmt.Errorf("field xpath '%s' resolves to no types", fieldXPath)
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	for i := range unique {
		for j := i + 1; j < len(unique); j++ {
			if !fieldTypesCompatible(unique[i], unique[j]) {
				return nil, fmt.Errorf("%w: field xpath '%s' selects incompatible types '%s' and '%s'", ErrFieldXPathIncompatibleTypes, fieldXPath, fieldTypeName(unique[i]), fieldTypeName(unique[j]))
			}
		}
	}
	return &model.SimpleType{
		Union:       &model.UnionType{},
		MemberTypes: unique,
	}, nil
}

func isDescendantOnlySteps(steps []xpath.Step) bool {
	if len(steps) == 0 {
		return false
	}
	sawDescendant := false
	for _, step := range steps {
		switch step.Axis {
		case xpath.AxisDescendantOrSelf:
			if !step.Test.Any {
				return false
			}
			sawDescendant = true
		case xpath.AxisSelf:
			if !step.Test.Any {
				return false
			}
		default:
			return false
		}
	}
	return sawDescendant
}

func uniqueElementDecls(decls []*model.ElementDecl) []*model.ElementDecl {
	if len(decls) == 0 {
		return nil
	}
	seen := make(map[model.QName]struct{}, len(decls))
	unique := make([]*model.ElementDecl, 0, len(decls))
	for _, decl := range decls {
		if decl == nil {
			continue
		}
		if _, ok := seen[decl.Name]; ok {
			continue
		}
		seen[decl.Name] = struct{}{}
		unique = append(unique, decl)
	}
	return unique
}
