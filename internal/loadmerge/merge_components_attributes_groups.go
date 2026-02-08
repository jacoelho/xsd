package loadmerge

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *mergeContext) mergeAttributeDecls() error {
	return mergeNamed(
		c.source.AttributeDecls,
		c.target.AttributeDecls,
		c.target.AttributeOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.AttributeOrigins, qname) },
		c.copyAttributeDecl,
		nil,
		nil,
		"attribute declaration",
	)
}

func (c *mergeContext) copyAttributeDecl(decl *types.AttributeDecl) *types.AttributeDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeAttributeGroups() error {
	insert := func(group *types.AttributeGroup) *types.AttributeGroup {
		groupCopy := group.Copy(c.opts)
		for _, attr := range groupCopy.Attributes {
			if attr.Form == types.FormDefault {
				if c.source.AttributeFormDefault == parser.Qualified {
					attr.Form = types.FormQualified
				} else {
					attr.Form = types.FormUnqualified
				}
			}
		}
		return groupCopy
	}
	return mergeNamed(
		c.source.AttributeGroups,
		c.target.AttributeGroups,
		c.target.AttributeGroupOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.AttributeGroupOrigins, qname) },
		insert,
		nil,
		nil,
		"attributeGroup",
	)
}

func (c *mergeContext) mergeGroups() error {
	return mergeNamed(
		c.source.Groups,
		c.target.Groups,
		c.target.GroupOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.GroupOrigins, qname) },
		func(group *types.ModelGroup) *types.ModelGroup { return group.Copy(c.opts) },
		nil,
		nil,
		"group",
	)
}

// normalizeAttributeForms explicitly sets the Form on attributes that have FormDefault
// based on the source schema's attributeFormDefault. This ensures that when types from
// imported or chameleon-included schemas are merged into a main schema, the attributes
// retain their original form semantics regardless of the main schema's attributeFormDefault.
func normalizeAttributeForms(complexType *types.ComplexType, sourceAttrFormDefault parser.Form) {
	normalizeAttr := func(attr *types.AttributeDecl) {
		if attr.Form == types.FormDefault {
			if sourceAttrFormDefault == parser.Qualified {
				attr.Form = types.FormQualified
			} else {
				attr.Form = types.FormUnqualified
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
