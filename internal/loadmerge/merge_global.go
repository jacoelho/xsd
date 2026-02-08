package loadmerge

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func existingGlobalDecls(sch *parser.Schema) map[globalDeclKey]struct{} {
	decls := make(map[globalDeclKey]struct{}, len(sch.GlobalDecls))
	for _, decl := range sch.GlobalDecls {
		decls[globalDeclKey{kind: decl.Kind, name: decl.Name}] = struct{}{}
	}
	return decls
}

func (c *mergeContext) mergeGlobalDecls(existing map[globalDeclKey]struct{}, insertAt int) {
	if c.source.GlobalDecls == nil {
		return
	}
	newDecls := make([]parser.GlobalDecl, 0, len(c.source.GlobalDecls))
	for _, decl := range c.source.GlobalDecls {
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
	if insertAt < 0 || insertAt > len(c.target.GlobalDecls) {
		insertAt = len(c.target.GlobalDecls)
	}
	c.target.GlobalDecls = insertGlobalDecls(c.target.GlobalDecls, insertAt, newDecls)
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

func (c *mergeContext) globalDeclExists(kind parser.GlobalDeclKind, name types.QName) bool {
	switch kind {
	case parser.GlobalDeclElement:
		_, ok := c.target.ElementDecls[name]
		return ok
	case parser.GlobalDeclType:
		_, ok := c.target.TypeDefs[name]
		return ok
	case parser.GlobalDeclAttribute:
		_, ok := c.target.AttributeDecls[name]
		return ok
	case parser.GlobalDeclAttributeGroup:
		_, ok := c.target.AttributeGroups[name]
		return ok
	case parser.GlobalDeclGroup:
		_, ok := c.target.Groups[name]
		return ok
	case parser.GlobalDeclNotation:
		_, ok := c.target.NotationDecls[name]
		return ok
	default:
		return false
	}
}
