package runtimebuild

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) initIDs() error {
	totalTypes := len(b.schema.BuiltinTypes) + len(b.schema.Types)
	if err := b.assembler.SetTypes(make([]runtime.Type, totalTypes+1)); err != nil {
		return err
	}

	complexCount := 0
	for _, entry := range b.schema.Types {
		if entry.Kind == schemair.TypeComplex {
			complexCount++
		}
	}
	complexCount++
	if err := b.assembler.SetComplexTypes(make([]runtime.ComplexType, complexCount+1)); err != nil {
		return err
	}
	b.anyTypeComplex = 1
	if err := b.assembler.SetElements(make([]runtime.Element, len(b.schema.Elements)+1)); err != nil {
		return err
	}
	if err := b.assembler.SetAttributes(make([]runtime.Attribute, len(b.schema.Attributes)+1)); err != nil {
		return err
	}

	symbols := b.rt.SymbolCount()
	if err := b.assembler.SetGlobalTypes(make([]runtime.TypeID, symbols+1)); err != nil {
		return err
	}
	if err := b.assembler.SetGlobalElements(make([]runtime.ElemID, symbols+1)); err != nil {
		return err
	}
	if err := b.assembler.SetGlobalAttributes(make([]runtime.AttrID, symbols+1)); err != nil {
		return err
	}

	if err := b.assembler.SetModels(runtime.ModelsBundle{
		DFA: make([]runtime.DFAModel, 1),
		NFA: make([]runtime.NFAModel, 1),
		All: make([]runtime.AllModel, 1),
	}); err != nil {
		return err
	}
	if err := b.assembler.SetAttrIndex(runtime.ComplexAttrIndex{
		Uses:       make([]runtime.AttrUse, 0),
		HashTables: nil,
	}); err != nil {
		return err
	}
	b.paths = make([]runtime.PathProgram, 1)
	return b.assembler.SetIdentityConstraints(make([]runtime.IdentityConstraint, 1))
}
