package preprocessor

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func existingGlobalDecls(graph *parser.SchemaGraph) map[globalDeclKey]struct{} {
	decls := make(map[globalDeclKey]struct{}, len(graph.GlobalDecls))
	for _, decl := range graph.GlobalDecls {
		decls[globalDeclKey{kind: decl.Kind, name: decl.Name}] = struct{}{}
	}
	return decls
}

func (c *mergeContext) mergeGlobalDecls(existing map[globalDeclKey]struct{}, insertAt int) {
	if c.sourceGraph.GlobalDecls == nil {
		return
	}
	newDecls := make([]parser.GlobalDecl, 0, len(c.sourceGraph.GlobalDecls))
	for _, decl := range c.sourceGraph.GlobalDecls {
		mappedName := c.remapQName(decl.Name)
		key := globalDeclKey{kind: decl.Kind, name: mappedName}
		if _, seen := existing[key]; seen {
			continue
		}
		if !c.globalDeclExists(decl.Kind, mappedName) {
			continue
		}
		newDecls = append(newDecls, parser.GlobalDecl{
			Kind: decl.Kind,
			Name: mappedName,
		})
		existing[key] = struct{}{}
	}
	if len(newDecls) == 0 {
		return
	}
	if insertAt < 0 || insertAt > len(c.targetGraph.GlobalDecls) {
		insertAt = len(c.targetGraph.GlobalDecls)
	}
	c.targetGraph.GlobalDecls = insertGlobalDecls(c.targetGraph.GlobalDecls, insertAt, newDecls)
}

func insertGlobalDecls(dst []parser.GlobalDecl, insertAt int, insert []parser.GlobalDecl) []parser.GlobalDecl {
	if len(insert) == 0 {
		return dst
	}
	if insertAt < 0 || insertAt > len(dst) {
		insertAt = len(dst)
	}
	merged := make([]parser.GlobalDecl, 0, len(dst)+len(insert))
	merged = append(merged, dst[:insertAt]...)
	merged = append(merged, insert...)
	merged = append(merged, dst[insertAt:]...)
	return merged
}

func (c *mergeContext) globalDeclExists(kind parser.GlobalDeclKind, name model.QName) bool {
	switch kind {
	case parser.GlobalDeclElement:
		_, ok := c.targetGraph.ElementDecls[name]
		return ok
	case parser.GlobalDeclType:
		_, ok := c.targetGraph.TypeDefs[name]
		return ok
	case parser.GlobalDeclAttribute:
		_, ok := c.targetGraph.AttributeDecls[name]
		return ok
	case parser.GlobalDeclAttributeGroup:
		_, ok := c.targetGraph.AttributeGroups[name]
		return ok
	case parser.GlobalDeclGroup:
		_, ok := c.targetGraph.Groups[name]
		return ok
	case parser.GlobalDeclNotation:
		_, ok := c.targetGraph.NotationDecls[name]
		return ok
	default:
		return false
	}
}
