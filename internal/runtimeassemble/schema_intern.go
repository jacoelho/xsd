package runtimeassemble

import (
	"github.com/jacoelho/xsd/internal/runtime"
	model "github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) internNamespaceConstraint(constraint model.NamespaceConstraint, list []model.NamespaceURI, target model.NamespaceURI) {
	if b == nil {
		return
	}
	switch constraint {
	case model.NSCTargetNamespace, model.NSCOther:
		b.internNamespace(target)
	case model.NSCList:
		for _, ns := range list {
			if ns == model.NamespaceTargetPlaceholder {
				b.internNamespace(target)
				continue
			}
			if ns == "" {
				continue
			}
			b.internNamespace(ns)
		}
	}
}

func (b *schemaBuilder) internWildcardNamespaces(particle model.Particle) {
	if particle == nil || b == nil {
		return
	}
	visited := make(map[*model.ModelGroup]bool)
	b.internWildcardNamespacesInParticle(particle, visited)
}

func (b *schemaBuilder) internWildcardNamespacesInParticle(particle model.Particle, visited map[*model.ModelGroup]bool) {
	if particle == nil {
		return
	}
	switch typed := particle.(type) {
	case *model.AnyElement:
		b.internNamespaceConstraint(typed.Namespace, typed.NamespaceList, typed.TargetNamespace)
	case *model.ModelGroup:
		if visited[typed] {
			return
		}
		visited[typed] = true
		for _, child := range typed.Particles {
			b.internWildcardNamespacesInParticle(child, visited)
		}
	case *model.GroupRef:
		if b.schema == nil {
			return
		}
		group := b.schema.Groups[typed.RefQName]
		if group == nil {
			return
		}
		b.internWildcardNamespacesInParticle(group, visited)
	}
}

func (b *schemaBuilder) addPath(program runtime.PathProgram) runtime.PathID {
	b.paths = append(b.paths, program)
	return runtime.PathID(len(b.paths) - 1)
}

func (b *schemaBuilder) internNamespace(ns model.NamespaceURI) runtime.NamespaceID {
	if b == nil {
		return 0
	}
	if b.err != nil {
		return 0
	}
	if b.rt != nil {
		if ns == "" {
			return b.rt.PredefNS.Empty
		}
		return b.rt.Namespaces.Lookup([]byte(ns))
	}
	if b.builder == nil {
		return 0
	}
	if ns == "" {
		id, err := b.builder.InternNamespace(nil)
		b.setError(err)
		return id
	}
	id, err := b.builder.InternNamespace([]byte(ns))
	b.setError(err)
	return id
}

func (b *schemaBuilder) internQName(qname model.QName) runtime.SymbolID {
	if b == nil {
		return 0
	}
	if b.err != nil {
		return 0
	}
	nsID := b.internNamespace(qname.Namespace)
	if nsID == 0 {
		return 0
	}
	if b.rt != nil {
		return b.rt.Symbols.Lookup(nsID, []byte(qname.Local))
	}
	if b.builder == nil {
		return 0
	}
	id, err := b.builder.InternSymbol(nsID, []byte(qname.Local))
	b.setError(err)
	return id
}

func (b *schemaBuilder) setError(err error) {
	if err == nil || b == nil || b.err != nil {
		return
	}
	b.err = err
}
