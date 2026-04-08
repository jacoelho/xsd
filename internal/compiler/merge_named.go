package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func orderedDeclNames[V any](graph *parser.SchemaGraph, kind parser.GlobalDeclKind, decls map[model.QName]V) []model.QName {
	if len(decls) == 0 {
		return nil
	}
	if graph == nil || len(graph.GlobalDecls) == 0 {
		return model.SortedMapKeys(decls)
	}

	names := make([]model.QName, 0, len(decls))
	for _, decl := range graph.GlobalDecls {
		if decl.Kind != kind {
			continue
		}
		if _, ok := decls[decl.Name]; ok {
			names = append(names, decl.Name)
		}
	}
	if len(names) == len(decls) {
		return names
	}

	seen := make(map[model.QName]struct{}, len(names))
	for _, name := range names {
		seen[name] = struct{}{}
	}

	extra := make([]model.QName, 0, len(decls)-len(names))
	for name := range decls {
		if _, ok := seen[name]; ok {
			continue
		}
		extra = append(extra, name)
	}
	model.SortInPlace(extra)
	return append(names, extra...)
}

func mergeNamed[V any](
	names []model.QName,
	source map[model.QName]V,
	target map[model.QName]V,
	targetOrigins map[model.QName]string,
	ensureTarget func() map[model.QName]V,
	ensureTargetOrigins func() map[model.QName]string,
	trackInsert func(model.QName),
	remap func(model.QName) model.QName,
	originFor func(model.QName) string,
	insert func(V) V,
	candidate func(V) V,
	equivalent func(existing V, candidate V) bool,
	kindName string,
) error {
	if insert == nil {
		insert = func(value V) V { return value }
	}
	if names == nil {
		names = model.SortedMapKeys(source)
	}
	for _, name := range names {
		value := source[name]
		targetQName := remap(name)
		origin := originFor(name)
		if existing, exists := target[targetQName]; exists {
			if targetOrigins[targetQName] == origin {
				continue
			}
			if equivalent != nil {
				cand := value
				if candidate != nil {
					cand = candidate(value)
				}
				if equivalent(existing, cand) {
					continue
				}
			}
			return fmt.Errorf("duplicate %s %s", kindName, targetQName)
		}
		if ensureTarget != nil {
			target = ensureTarget()
		}
		target[targetQName] = insert(value)
		if ensureTargetOrigins != nil {
			targetOrigins = ensureTargetOrigins()
		}
		targetOrigins[targetQName] = origin
		if trackInsert != nil {
			trackInsert(targetQName)
		}
	}
	return nil
}

func mergeNamedGlobalDecl[V any](
	c *mergeContext,
	kind parser.GlobalDeclKind,
	source map[model.QName]V,
	target map[model.QName]V,
	targetOrigins map[model.QName]string,
	ensureTarget func() map[model.QName]V,
	ensureTargetOrigins func() map[model.QName]string,
	sourceOrigins map[model.QName]string,
	insert func(V) V,
	kindName string,
) error {
	return mergeNamed(
		orderedDeclNames(c.sourceGraph, kind, source),
		source,
		target,
		targetOrigins,
		ensureTarget,
		ensureTargetOrigins,
		func(name model.QName) { c.recordInsertedGlobalDecl(kind, name) },
		c.remapQName,
		func(name model.QName) string { return c.originFor(sourceOrigins, name) },
		insert,
		nil,
		nil,
		kindName,
	)
}
