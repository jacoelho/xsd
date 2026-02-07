package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) initIDs() {
	builtin := builtinTypeNames()
	totalTypes := len(builtin) + len(b.registry.TypeOrder)
	b.rt.Types = make([]runtime.Type, totalTypes+1)

	complexCount := 0
	for _, entry := range b.registry.TypeOrder {
		if _, ok := types.AsComplexType(entry.Type); ok {
			complexCount++
		}
	}
	complexCount++
	b.rt.ComplexTypes = make([]runtime.ComplexType, complexCount+1)
	b.anyTypeComplex = 1
	b.rt.Elements = make([]runtime.Element, len(b.registry.ElementOrder)+1)
	b.rt.Attributes = make([]runtime.Attribute, len(b.registry.AttributeOrder)+1)

	nextType := runtime.TypeID(1)
	for _, name := range builtin {
		b.builtinIDs[name] = nextType
		nextType++
	}
	for _, entry := range b.registry.TypeOrder {
		b.typeIDs[entry.ID] = nextType
		nextType++
	}

	nextElem := runtime.ElemID(1)
	for _, entry := range b.registry.ElementOrder {
		b.elemIDs[entry.ID] = nextElem
		nextElem++
	}

	nextAttr := runtime.AttrID(1)
	for _, entry := range b.registry.AttributeOrder {
		b.attrIDs[entry.ID] = nextAttr
		nextAttr++
	}

	b.rt.GlobalTypes = make([]runtime.TypeID, b.rt.Symbols.Count()+1)
	b.rt.GlobalElements = make([]runtime.ElemID, b.rt.Symbols.Count()+1)
	b.rt.GlobalAttributes = make([]runtime.AttrID, b.rt.Symbols.Count()+1)

	b.rt.Models = runtime.ModelsBundle{
		DFA: make([]runtime.DFAModel, 1),
		NFA: make([]runtime.NFAModel, 1),
		All: make([]runtime.AllModel, 1),
	}
	b.rt.AttrIndex = runtime.ComplexAttrIndex{
		Uses:       make([]runtime.AttrUse, 0),
		HashTables: nil,
	}
	b.paths = make([]runtime.PathProgram, 1)
	b.rt.ICs = make([]runtime.IdentityConstraint, 1)
}
