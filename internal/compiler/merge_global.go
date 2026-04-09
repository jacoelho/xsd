package compiler

import (
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

const inlineInsertedQNameCap = 4

type insertedQNameSet struct {
	inline [inlineInsertedQNameCap]model.QName
	n      int
	large  map[model.QName]struct{}
}

func (s *insertedQNameSet) len() int {
	if s == nil {
		return 0
	}
	if s.large != nil {
		return len(s.large)
	}
	return s.n
}

func (s *insertedQNameSet) contains(name model.QName) bool {
	if s == nil {
		return false
	}
	if s.large != nil {
		_, ok := s.large[name]
		return ok
	}
	for i := 0; i < s.n; i++ {
		if s.inline[i] == name {
			return true
		}
	}
	return false
}

func (s *insertedQNameSet) add(name model.QName, expectedCap int) {
	if s == nil {
		return
	}
	if s.large != nil {
		s.large[name] = struct{}{}
		return
	}
	for i := 0; i < s.n; i++ {
		if s.inline[i] == name {
			return
		}
	}
	if s.n < len(s.inline) {
		s.inline[s.n] = name
		s.n++
		return
	}
	if expectedCap < s.n+1 {
		expectedCap = s.n + 1
	}
	large := make(map[model.QName]struct{}, expectedCap)
	for i := 0; i < s.n; i++ {
		large[s.inline[i]] = struct{}{}
	}
	large[name] = struct{}{}
	s.large = large
	s.n = 0
}

func (c *mergeContext) expectedInsertedGlobalDeclCount(kind parser.GlobalDeclKind) int {
	if c == nil || c.sourceGraph == nil || c.targetGraph == nil {
		return 0
	}
	if kind < parser.GlobalDeclElement || kind > parser.GlobalDeclNotation {
		return 0
	}
	if c.expectedInsertCountsCached[kind] {
		return c.expectedInsertCounts[kind]
	}

	count := 0
	switch kind {
	case parser.GlobalDeclElement:
		count = expectedQNameInsertCount(c.sourceGraph.ElementDecls, c.targetGraph.ElementDecls, c.remapQName)
	case parser.GlobalDeclType:
		count = expectedQNameInsertCount(c.sourceGraph.TypeDefs, c.targetGraph.TypeDefs, c.remapQName)
	case parser.GlobalDeclAttribute:
		count = expectedQNameInsertCount(c.sourceGraph.AttributeDecls, c.targetGraph.AttributeDecls, c.remapQName)
	case parser.GlobalDeclAttributeGroup:
		count = expectedQNameInsertCount(c.sourceGraph.AttributeGroups, c.targetGraph.AttributeGroups, c.remapQName)
	case parser.GlobalDeclGroup:
		count = expectedQNameInsertCount(c.sourceGraph.Groups, c.targetGraph.Groups, c.remapQName)
	case parser.GlobalDeclNotation:
		count = expectedQNameInsertCount(c.sourceGraph.NotationDecls, c.targetGraph.NotationDecls, c.remapQName)
	}
	c.expectedInsertCounts[kind] = count
	c.expectedInsertCountsCached[kind] = true
	return count
}

func (c *mergeContext) insertedGlobalDeclSet(kind parser.GlobalDeclKind) *insertedQNameSet {
	if c == nil || kind < parser.GlobalDeclElement || kind > parser.GlobalDeclNotation {
		return nil
	}
	return &c.insertedGlobalDecls[kind]
}

func (c *mergeContext) recordInsertedGlobalDecl(kind parser.GlobalDeclKind, name model.QName) {
	if c == nil {
		return
	}
	c.insertedGlobalDeclSet(kind).add(name, c.expectedInsertedGlobalDeclCount(kind))
}

func (c *mergeContext) insertedGlobalDeclCount() int {
	if c == nil {
		return 0
	}
	total := 0
	for _, inserted := range c.insertedGlobalDecls {
		total += inserted.len()
	}
	return total
}

type insertedGlobalDeclKey struct {
	kind parser.GlobalDeclKind
	name model.QName
}

func (c *mergeContext) insertedGlobalDeclsInOrder() []parser.GlobalDecl {
	if c == nil || c.sourceGraph == nil || len(c.sourceGraph.GlobalDecls) == 0 {
		return nil
	}

	decls := make([]parser.GlobalDecl, 0, c.insertedGlobalDeclCount())
	seen := make(map[insertedGlobalDeclKey]struct{}, c.insertedGlobalDeclCount())
	for _, decl := range c.sourceGraph.GlobalDecls {
		mappedName := c.remapQName(decl.Name)
		if !c.insertedGlobalDecls[decl.Kind].contains(mappedName) {
			continue
		}

		key := insertedGlobalDeclKey{
			kind: decl.Kind,
			name: mappedName,
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		decls = append(decls, parser.GlobalDecl{
			Kind: decl.Kind,
			Name: mappedName,
		})
	}
	return decls
}

func (c *mergeContext) mergeGlobalDecls(insertAt int) {
	if c.sourceGraph.GlobalDecls == nil {
		return
	}
	newDecls := c.insertedGlobalDeclsInOrder()
	if len(newDecls) == 0 {
		return
	}
	if insertAt < 0 || insertAt > len(c.targetGraph.GlobalDecls) {
		insertAt = len(c.targetGraph.GlobalDecls)
	}
	if insertAt == len(c.targetGraph.GlobalDecls) {
		targetDecls := slices.Clip(c.targetGraph.GlobalDecls)
		targetDecls = slices.Grow(targetDecls, len(newDecls))
		targetDecls = append(targetDecls, newDecls...)
		c.targetGraph.GlobalDecls = targetDecls
		return
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
