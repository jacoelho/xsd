package runtimeassemble

import (
	"maps"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimeids"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) initIDs() error {
	plan, err := runtimeids.Build(b.registry)
	if err != nil {
		return err
	}
	totalTypes := len(plan.BuiltinTypeNames) + len(b.registry.TypeOrder)
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

	maps.Copy(b.builtinIDs, plan.BuiltinTypeIDs)
	maps.Copy(b.typeIDs, plan.TypeIDs)

	maps.Copy(b.elemIDs, plan.ElementIDs)

	maps.Copy(b.attrIDs, plan.AttributeIDs)

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
