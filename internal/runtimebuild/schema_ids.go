package runtimebuild

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) initIDs() error {
	totalTypes := len(b.schema.BuiltinTypes) + len(b.schema.Types)
	b.rt.Types = make([]runtime.Type, totalTypes+1)

	complexCount := 0
	for _, entry := range b.schema.Types {
		if entry.Kind == schemair.TypeComplex {
			complexCount++
		}
	}
	complexCount++
	b.rt.ComplexTypes = make([]runtime.ComplexType, complexCount+1)
	b.anyTypeComplex = 1
	b.rt.Elements = make([]runtime.Element, len(b.schema.Elements)+1)
	b.rt.Attributes = make([]runtime.Attribute, len(b.schema.Attributes)+1)

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
	return nil
}
