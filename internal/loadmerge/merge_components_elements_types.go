package loadmerge

import "github.com/jacoelho/xsd/internal/model"

func (c *mergeContext) mergeElementDecls() error {
	return mergeNamed(
		c.source.ElementDecls,
		c.target.ElementDecls,
		c.target.ElementOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.source.ElementOrigins, qname) },
		c.elementDeclForInsert,
		c.elementDeclCandidate,
		elementDeclEquivalent,
		"element declaration",
	)
}

func (c *mergeContext) elementDeclCandidate(decl *model.ElementDecl) *model.ElementDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	if c.needsNamespaceRemap {
		return decl.Copy(c.opts)
	}
	return decl
}

func (c *mergeContext) elementDeclForInsert(decl *model.ElementDecl) *model.ElementDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeTypeDefs() error {
	return mergeNamed(
		c.source.TypeDefs,
		c.target.TypeDefs,
		c.target.TypeOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.source.TypeOrigins, qname) },
		c.copyType,
		nil,
		nil,
		"type definition",
	)
}

func (c *mergeContext) copyType(typ model.Type) model.Type {
	copiedType := model.CopyType(typ, c.opts)
	if complexType, ok := copiedType.(*model.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copiedType
}
