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
	insert := func(typ model.Type) model.Type {
		if c.isImport {
			return c.copyTypeForImport(typ)
		}
		return c.copyTypeForInclude(typ)
	}
	return mergeNamed(
		c.source.TypeDefs,
		c.target.TypeDefs,
		c.target.TypeOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.source.TypeOrigins, qname) },
		insert,
		nil,
		nil,
		"type definition",
	)
}

func (c *mergeContext) copyTypeForImport(typ model.Type) model.Type {
	copied := model.CopyType(typ, c.opts)
	if complexType, ok := copied.(*model.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copied
}

func (c *mergeContext) copyTypeForInclude(typ model.Type) model.Type {
	copiedType := model.CopyType(typ, c.opts)
	if complexType, ok := copiedType.(*model.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copiedType
}
