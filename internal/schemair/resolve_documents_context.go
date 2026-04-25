package schemair

import (
	"slices"

	ast "github.com/jacoelho/xsd/internal/schemaast"
)

func normalizeDocumentContextIDs(docs []ast.SchemaDocument) ([]ast.SchemaDocument, map[ast.NamespaceContextID]ast.NamespaceContext) {
	out := make([]ast.SchemaDocument, len(docs))
	contexts := make(map[ast.NamespaceContextID]ast.NamespaceContext)
	next := ast.NamespaceContextID(0)
	for i := range docs {
		doc := docs[i]
		idMap := make(map[ast.NamespaceContextID]ast.NamespaceContextID, len(doc.NamespaceContexts))
		for oldID, context := range doc.NamespaceContexts {
			newID := next
			next++
			idMap[ast.NamespaceContextID(oldID)] = newID
			contexts[newID] = cloneNamespaceContext(context)
		}
		out[i] = cloneSchemaDocumentWithContextIDs(doc, idMap)
	}
	return out, contexts
}

func cloneNamespaceContext(context ast.NamespaceContext) ast.NamespaceContext {
	return ast.NamespaceContext{Bindings: slices.Clone(context.Bindings)}
}

func cloneSchemaDocumentWithContextIDs(doc ast.SchemaDocument, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.SchemaDocument {
	out := doc
	out.NamespaceContexts = make([]ast.NamespaceContext, len(doc.NamespaceContexts))
	for i := range doc.NamespaceContexts {
		out.NamespaceContexts[i] = cloneNamespaceContext(doc.NamespaceContexts[i])
	}
	out.Decls = make([]ast.TopLevelDecl, len(doc.Decls))
	for i := range doc.Decls {
		out.Decls[i] = cloneTopLevelDeclWithContextIDs(doc.Decls[i], ids)
	}
	out.Directives = slices.Clone(doc.Directives)
	out.Imports = slices.Clone(doc.Imports)
	out.Includes = slices.Clone(doc.Includes)
	return out
}

func cloneTopLevelDeclWithContextIDs(decl ast.TopLevelDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.TopLevelDecl {
	out := decl
	if decl.SimpleType != nil {
		out.SimpleType = cloneSimpleTypeDeclWithContextIDs(decl.SimpleType, ids)
	}
	if decl.ComplexType != nil {
		out.ComplexType = cloneComplexTypeDeclWithContextIDs(decl.ComplexType, ids)
	}
	if decl.Element != nil {
		out.Element = cloneElementDeclWithContextIDs(decl.Element, ids)
	}
	if decl.Attribute != nil {
		out.Attribute = cloneAttributeDeclWithContextIDs(decl.Attribute, ids)
	}
	if decl.Group != nil {
		out.Group = cloneGroupDeclWithContextIDs(decl.Group, ids)
	}
	if decl.AttributeGroup != nil {
		out.AttributeGroup = cloneAttributeGroupDeclWithContextIDs(decl.AttributeGroup, ids)
	}
	if decl.Notation != nil {
		notation := *decl.Notation
		out.Notation = &notation
	}
	return out
}

func cloneSimpleTypeDeclWithContextIDs(decl *ast.SimpleTypeDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.SimpleTypeDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.MemberTypes = slices.Clone(decl.MemberTypes)
	out.Facets = cloneFacetDeclsWithContextIDs(decl.Facets, ids)
	out.InlineBase = cloneSimpleTypeDeclWithContextIDs(decl.InlineBase, ids)
	out.InlineItem = cloneSimpleTypeDeclWithContextIDs(decl.InlineItem, ids)
	out.InlineMembers = make([]ast.SimpleTypeDecl, len(decl.InlineMembers))
	for i := range decl.InlineMembers {
		copied := cloneSimpleTypeDeclWithContextIDs(&decl.InlineMembers[i], ids)
		if copied != nil {
			out.InlineMembers[i] = *copied
		}
	}
	return &out
}

func cloneComplexTypeDeclWithContextIDs(decl *ast.ComplexTypeDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.ComplexTypeDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.Attributes = make([]ast.AttributeUseDecl, len(decl.Attributes))
	for i := range decl.Attributes {
		out.Attributes[i] = cloneAttributeUseDeclWithContextIDs(decl.Attributes[i], ids)
	}
	out.AttributeGroups = slices.Clone(decl.AttributeGroups)
	out.AnyAttribute = cloneWildcardDecl(decl.AnyAttribute)
	out.Particle = cloneParticleDeclWithContextIDs(decl.Particle, ids)
	out.SimpleType = cloneSimpleTypeDeclWithContextIDs(decl.SimpleType, ids)
	out.SimpleFacets = cloneFacetDeclsWithContextIDs(decl.SimpleFacets, ids)
	return &out
}

func cloneElementDeclWithContextIDs(decl *ast.ElementDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.ElementDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.NamespaceContextID = remapContextID(decl.NamespaceContextID, ids)
	out.Type = cloneTypeUseWithContextIDs(decl.Type, ids)
	out.Default = cloneValueConstraintWithContextID(decl.Default, ids)
	out.Fixed = cloneValueConstraintWithContextID(decl.Fixed, ids)
	out.Identity = make([]ast.IdentityDecl, len(decl.Identity))
	for i := range decl.Identity {
		out.Identity[i] = cloneIdentityDeclWithContextID(decl.Identity[i], ids)
	}
	return &out
}

func cloneAttributeDeclWithContextIDs(decl *ast.AttributeDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.AttributeDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.NamespaceContextID = remapContextID(decl.NamespaceContextID, ids)
	out.Type = cloneTypeUseWithContextIDs(decl.Type, ids)
	out.Default = cloneValueConstraintWithContextID(decl.Default, ids)
	out.Fixed = cloneValueConstraintWithContextID(decl.Fixed, ids)
	return &out
}

func cloneAttributeUseDeclWithContextIDs(decl ast.AttributeUseDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.AttributeUseDecl {
	out := decl
	out.Attribute = cloneAttributeDeclWithContextIDs(decl.Attribute, ids)
	return out
}

func cloneGroupDeclWithContextIDs(decl *ast.GroupDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.GroupDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.Particle = cloneParticleDeclWithContextIDs(decl.Particle, ids)
	return &out
}

func cloneAttributeGroupDeclWithContextIDs(decl *ast.AttributeGroupDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.AttributeGroupDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.Attributes = make([]ast.AttributeUseDecl, len(decl.Attributes))
	for i := range decl.Attributes {
		out.Attributes[i] = cloneAttributeUseDeclWithContextIDs(decl.Attributes[i], ids)
	}
	out.AttributeGroups = slices.Clone(decl.AttributeGroups)
	out.AnyAttribute = cloneWildcardDecl(decl.AnyAttribute)
	return &out
}

func cloneParticleDeclWithContextIDs(decl *ast.ParticleDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) *ast.ParticleDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.Element = cloneElementDeclWithContextIDs(decl.Element, ids)
	out.Wildcard = cloneWildcardDecl(decl.Wildcard)
	out.Children = make([]ast.ParticleDecl, len(decl.Children))
	for i := range decl.Children {
		copied := cloneParticleDeclWithContextIDs(&decl.Children[i], ids)
		if copied != nil {
			out.Children[i] = *copied
		}
	}
	return &out
}

func cloneWildcardDecl(decl *ast.WildcardDecl) *ast.WildcardDecl {
	if decl == nil {
		return nil
	}
	out := *decl
	out.NamespaceList = slices.Clone(decl.NamespaceList)
	return &out
}

func cloneTypeUseWithContextIDs(use ast.TypeUse, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.TypeUse {
	return ast.TypeUse{
		Name:    use.Name,
		Simple:  cloneSimpleTypeDeclWithContextIDs(use.Simple, ids),
		Complex: cloneComplexTypeDeclWithContextIDs(use.Complex, ids),
	}
}

func cloneFacetDeclsWithContextIDs(facets []ast.FacetDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) []ast.FacetDecl {
	out := make([]ast.FacetDecl, len(facets))
	for i := range facets {
		out[i] = facets[i]
		out[i].NamespaceContextID = remapContextID(facets[i].NamespaceContextID, ids)
	}
	return out
}

func cloneValueConstraintWithContextID(value ast.ValueConstraintDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.ValueConstraintDecl {
	value.NamespaceContextID = remapContextID(value.NamespaceContextID, ids)
	return value
}

func cloneIdentityDeclWithContextID(decl ast.IdentityDecl, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.IdentityDecl {
	decl.NamespaceContextID = remapContextID(decl.NamespaceContextID, ids)
	decl.Fields = slices.Clone(decl.Fields)
	return decl
}

func remapContextID(id ast.NamespaceContextID, ids map[ast.NamespaceContextID]ast.NamespaceContextID) ast.NamespaceContextID {
	if mapped, ok := ids[id]; ok {
		return mapped
	}
	return id
}
