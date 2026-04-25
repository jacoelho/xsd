package runtimebuild

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) initSymbols() error {
	if b.builder == nil {
		return fmt.Errorf("runtime build: symbol builder missing")
	}
	if b.schema == nil {
		return fmt.Errorf("runtime build: schema ir is nil")
	}
	for _, op := range b.schema.RuntimeNames.Ops {
		switch op.Kind {
		case schemair.RuntimeNameSymbol:
			b.internIRName(op.Name)
		case schemair.RuntimeNameNamespace:
			b.internNamespace(op.Namespace)
		default:
			return fmt.Errorf("runtime build: unknown runtime name op %d", op.Kind)
		}
	}
	for _, name := range b.schema.RuntimeNames.Notations {
		if id := b.internIRName(name); id != 0 {
			b.notations = append(b.notations, id)
		}
	}
	if len(b.notations) > 1 {
		slices.Sort(b.notations)
		b.notations = slices.Compact(b.notations)
	}
	return b.err
}

func (b *schemaBuilder) internIRName(name schemair.Name) runtime.SymbolID {
	return b.internName(name)
}
