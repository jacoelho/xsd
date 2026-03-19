package merge

import "github.com/jacoelho/xsd/internal/model"

func (c *mergeContext) mergeElementDecls() error {
	return mergeNamed(
		c.sourceGraph.ElementDecls,
		c.targetGraph.ElementDecls,
		c.targetMeta.ElementOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.sourceMeta.ElementOrigins, qname) },
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
		declCopy.SourceNamespace = c.sourceMeta.TargetNamespace
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
		declCopy.SourceNamespace = c.sourceMeta.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeTypeDefs() error {
	return mergeNamed(
		c.sourceGraph.TypeDefs,
		c.targetGraph.TypeDefs,
		c.targetMeta.TypeOrigins,
		c.remapQName,
		func(qname model.QName) string { return c.originFor(c.sourceMeta.TypeOrigins, qname) },
		c.copyType,
		nil,
		nil,
		"type definition",
	)
}

func (c *mergeContext) copyType(typ model.Type) model.Type {
	copiedType := model.CopyType(typ, c.opts)
	if complexType, ok := copiedType.(*model.ComplexType); ok {
		normalizeAttributeForms(complexType, c.sourceMeta.AttributeFormDefault)
	}
	return copiedType
}
