package loadmerge

import "github.com/jacoelho/xsd/internal/types"

func (c *mergeContext) mergeElementDecls() error {
	return mergeNamed(
		c.source.ElementDecls,
		c.target.ElementDecls,
		c.target.ElementOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.ElementOrigins, qname) },
		c.elementDeclForInsert,
		c.elementDeclCandidate,
		elementDeclEquivalent,
		"element declaration",
	)
}

func (c *mergeContext) elementDeclCandidate(decl *types.ElementDecl) *types.ElementDecl {
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

func (c *mergeContext) elementDeclForInsert(decl *types.ElementDecl) *types.ElementDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeTypeDefs() error {
	insert := func(typ types.Type) types.Type {
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
		func(qname types.QName) string { return c.originFor(c.source.TypeOrigins, qname) },
		insert,
		nil,
		nil,
		"type definition",
	)
}

func (c *mergeContext) copyTypeForImport(typ types.Type) types.Type {
	copied := types.CopyType(typ, c.opts)
	if complexType, ok := copied.(*types.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copied
}

func (c *mergeContext) copyTypeForInclude(typ types.Type) types.Type {
	copiedType := types.CopyType(typ, c.opts)
	if complexType, ok := copiedType.(*types.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copiedType
}
