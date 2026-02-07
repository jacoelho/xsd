package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (b *schemaBuilder) buildAncestors() error {
	if b.rt == nil {
		return fmt.Errorf("runtime build: schema missing for ancestors")
	}
	typeCount := len(b.rt.Types)
	ids := make([]runtime.TypeID, 0, typeCount)
	masks := make([]runtime.DerivationMethod, 0, typeCount)

	for id := runtime.TypeID(1); int(id) < typeCount; id++ {
		typ := b.rt.Types[id]
		offset := uint32(len(ids))

		var err error
		ids, masks, err = b.appendAncestors(id, typ, ids, masks)
		if err != nil {
			return err
		}

		typ.AncOff = offset
		typ.AncLen = uint32(len(ids)) - offset
		typ.AncMaskOff = typ.AncOff
		b.rt.Types[id] = typ
	}

	b.rt.Ancestors = runtime.TypeAncestors{IDs: ids, Masks: masks}
	return nil
}

func (b *schemaBuilder) appendAncestors(id runtime.TypeID, typ runtime.Type, ids []runtime.TypeID, masks []runtime.DerivationMethod) ([]runtime.TypeID, []runtime.DerivationMethod, error) {
	if b == nil {
		return ids, masks, fmt.Errorf("runtime build: schema builder missing")
	}
	if id == 0 || b.rt == nil {
		return ids, masks, fmt.Errorf("runtime build: invalid type ID for ancestors")
	}
	typeCount := len(b.rt.Types)
	cumulative := runtime.DerivationMethod(0)
	base := typ.Base
	current := typ
	visited := make(map[runtime.TypeID]bool)

	for base != 0 {
		if current.Derivation == runtime.DerNone {
			return ids, masks, fmt.Errorf("runtime build: type %d missing derivation method", id)
		}
		if int(base) >= typeCount {
			return ids, masks, fmt.Errorf("runtime build: ancestor type %d out of range", base)
		}
		if visited[base] {
			return ids, masks, fmt.Errorf("runtime build: type derivation cycle at %d", base)
		}
		visited[base] = true

		cumulative |= current.Derivation
		ids = append(ids, base)
		masks = append(masks, cumulative)

		current = b.rt.Types[base]
		base = current.Base
	}

	return ids, masks, nil
}
