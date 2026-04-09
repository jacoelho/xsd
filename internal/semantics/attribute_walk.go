package semantics

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type attributeVisitFunc func(attr *model.AttributeDecl, fromGroup bool) error

func walkComplexTypeLocalAttributes(
	schema *parser.Schema,
	ct *model.ComplexType,
	opts analysis.AttributeGroupWalkOptions,
	visit attributeVisitFunc,
) error {
	if ct == nil || visit == nil {
		return nil
	}
	if err := walkAttributeDecls(ct.Attributes(), false, visit); err != nil {
		return err
	}
	if err := walkAttributeGroupAttributes(schema, ct.AttrGroups, opts, visit); err != nil {
		return err
	}

	content := ct.Content()
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := walkAttributeDecls(ext.Attributes, false, visit); err != nil {
			return err
		}
		if err := walkAttributeGroupAttributes(schema, ext.AttrGroups, opts, visit); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := walkAttributeDecls(restr.Attributes, false, visit); err != nil {
			return err
		}
		if err := walkAttributeGroupAttributes(schema, restr.AttrGroups, opts, visit); err != nil {
			return err
		}
	}
	return nil
}

func walkComplexTypeAttributeChain(
	schema *parser.Schema,
	ct *model.ComplexType,
	opts analysis.AttributeGroupWalkOptions,
	visit func(current *model.ComplexType, attr *model.AttributeDecl, fromGroup bool) error,
) error {
	if ct == nil || visit == nil {
		return nil
	}
	chain := CollectComplexTypeChain(schema, ct, ComplexTypeChainExplicitBaseOnly)
	for i := len(chain) - 1; i >= 0; i-- {
		current := chain[i]
		if err := walkComplexTypeLocalAttributes(schema, current, opts, func(attr *model.AttributeDecl, fromGroup bool) error {
			return visit(current, attr, fromGroup)
		}); err != nil {
			return err
		}
	}
	return nil
}

func walkAttributeGroupAttributes(
	schema *parser.Schema,
	refs []model.QName,
	opts analysis.AttributeGroupWalkOptions,
	visit attributeVisitFunc,
) error {
	if visit == nil {
		return nil
	}
	ctx := analysis.NewAttributeGroupContext(schema, opts)
	return ctx.Walk(refs, func(_ model.QName, group *model.AttributeGroup) error {
		if group == nil {
			return nil
		}
		return walkAttributeDecls(group.Attributes, true, visit)
	})
}

func walkAttributeDecls(attrs []*model.AttributeDecl, fromGroup bool, visit attributeVisitFunc) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		if err := visit(attr, fromGroup); err != nil {
			return err
		}
	}
	return nil
}
