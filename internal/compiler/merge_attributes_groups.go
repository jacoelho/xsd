package compiler

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func (c *mergeContext) mergeAttributeDecls() error {
	return mergeNamed(
		c.sourceGraph.AttributeDecls,
		c.targetGraph.AttributeDecls,
		c.targetMeta.AttributeOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.sourceMeta.AttributeOrigins, qname) },
		c.copyAttributeDecl,
		nil,
		nil,
		"attribute declaration",
	)
}

func (c *mergeContext) copyAttributeDecl(decl *model.AttributeDecl) *model.AttributeDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.sourceMeta.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeAttributeGroups() error {
	insert := func(group *model.AttributeGroup) *model.AttributeGroup {
		groupCopy := group.Copy(c.opts)
		for _, attr := range groupCopy.Attributes {
			if attr.Form == model.FormDefault {
				if c.sourceMeta.AttributeFormDefault == parser.Qualified {
					attr.Form = model.FormQualified
				} else {
					attr.Form = model.FormUnqualified
				}
			}
		}
		return groupCopy
	}
	return mergeNamed(
		c.sourceGraph.AttributeGroups,
		c.targetGraph.AttributeGroups,
		c.targetMeta.AttributeGroupOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.sourceMeta.AttributeGroupOrigins, qname) },
		insert,
		nil,
		nil,
		"attributeGroup",
	)
}

func (c *mergeContext) mergeGroups() error {
	return mergeNamed(
		c.sourceGraph.Groups,
		c.targetGraph.Groups,
		c.targetMeta.GroupOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.sourceMeta.GroupOrigins, qname) },
		func(group *model.ModelGroup) *model.ModelGroup { return group.Copy(c.opts) },
		nil,
		nil,
		"group",
	)
}

// normalizeAttributeForms explicitly sets the Form on attributes that have FormDefault
// based on the source schema's attributeFormDefault. This ensures that when types from
// imported or chameleon-included schemas are merged into a main schema, the attributes
// retain their original form semantics regardless of the main schema's attributeFormDefault.
func normalizeAttributeForms(complexType *model.ComplexType, sourceAttrFormDefault parser.Form) {
	normalizeAttr := func(attr *model.AttributeDecl) {
		if attr.Form == model.FormDefault {
			if sourceAttrFormDefault == parser.Qualified {
				attr.Form = model.FormQualified
			} else {
				attr.Form = model.FormUnqualified
			}
		}
	}

	for _, attr := range complexType.Attributes() {
		normalizeAttr(attr)
	}

	content := complexType.Content()
	if ext := content.ExtensionDef(); ext != nil {
		for _, attr := range ext.Attributes {
			normalizeAttr(attr)
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		for _, attr := range restr.Attributes {
			normalizeAttr(attr)
		}
	}
}
