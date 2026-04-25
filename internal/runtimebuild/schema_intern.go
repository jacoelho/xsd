package runtimebuild

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) addPath(program runtime.PathProgram) runtime.PathID {
	b.paths = append(b.paths, program)
	return runtime.PathID(len(b.paths) - 1)
}

func (b *schemaBuilder) internNamespace(ns string) runtime.NamespaceID {
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

func (b *schemaBuilder) internName(name schemair.Name) runtime.SymbolID {
	if b == nil {
		return 0
	}
	if b.err != nil {
		return 0
	}
	nsID := b.internNamespace(name.Namespace)
	if nsID == 0 {
		return 0
	}
	if b.rt != nil {
		return b.rt.Symbols.Lookup(nsID, []byte(name.Local))
	}
	if b.builder == nil {
		return 0
	}
	id, err := b.builder.InternSymbol(nsID, []byte(name.Local))
	b.setError(err)
	return id
}

func (b *schemaBuilder) setError(err error) {
	if err == nil || b == nil || b.err != nil {
		return
	}
	b.err = err
}
