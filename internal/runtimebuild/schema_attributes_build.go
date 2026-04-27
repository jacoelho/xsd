package runtimebuild

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) buildAttributes() error {
	if err := b.buildGlobalAttributes(); err != nil {
		return err
	}
	return b.buildComplexTypeAttributeIndexes()
}

func (b *schemaBuilder) buildGlobalAttributes() error {
	if b.schema == nil {
		return fmt.Errorf("runtime build: schema ir is nil")
	}
	for _, entry := range b.schema.Attributes {
		id := runtime.AttrID(entry.ID)
		if id == 0 {
			return fmt.Errorf("runtime build: attribute %s missing runtime ID", formatIRName(entry.Name))
		}
		sym, err := b.lookupIRSymbol(entry.Name)
		if err != nil {
			return err
		}
		attr := runtime.Attribute{Name: sym}
		vid, ok := b.validatorForIRTypeRef(entry.TypeDecl)
		if !ok || vid == 0 {
			return fmt.Errorf("runtime build: attribute %s missing validator", formatIRName(entry.Name))
		}
		attr.Validator = vid

		if def, ok := b.artifacts.AttributeDefaults[entry.ID]; ok {
			attr.Default = def.Ref
			attr.DefaultKey = def.Key
			attr.DefaultMember = def.Member
		}
		if fixed, ok := b.artifacts.AttributeFixed[entry.ID]; ok {
			attr.Fixed = fixed.Ref
			attr.FixedKey = fixed.Key
			attr.FixedMember = fixed.Member
		}
		if err := b.assembler.SetAttribute(id, attr); err != nil {
			return err
		}
		if entry.Global {
			if err := b.assembler.SetGlobalAttribute(sym, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *schemaBuilder) validatorForIRTypeRef(ref schemair.TypeRef) (runtime.ValidatorID, bool) {
	if ref.IsZero() {
		return 0, false
	}
	if ref.IsBuiltin() {
		id := b.artifacts.BuiltinValidators[ref.TypeName().Local]
		return id, id != 0
	}
	id := b.artifacts.TypeValidators[ref.TypeID()]
	return id, id != 0
}

func (b *schemaBuilder) buildComplexTypeAttributeIndexes() error {
	for _, plan := range b.schema.ComplexTypes {
		typeID := b.userTypeRuntimeID(plan.TypeDecl)
		complexID := b.complexIDs[typeID]
		if complexID == 0 {
			continue
		}
		complexType, ok := b.rt.ComplexType(complexID)
		if !ok {
			return fmt.Errorf("runtime build: complex type %d out of range", complexID)
		}

		uses, err := b.collectAttrUses(plan.Attrs)
		if err != nil {
			return err
		}
		if plan.AnyAttr != 0 {
			id, err := b.addWildcardFromIR(plan.AnyAttr)
			if err != nil {
				return err
			}
			complexType.AnyAttr = id
		}
		if len(uses) > 0 {
			off := uint32(b.rt.AttributeUseCount())
			var mode runtime.AttrIndexMode
			hashTable := uint32(0)
			switch {
			case len(uses) <= attrIndexLinearLimit:
				mode = runtime.AttrIndexSmallLinear
			case len(uses) <= attrIndexBinaryLimit:
				slices.SortFunc(uses, func(a, b runtime.AttrUse) int {
					return cmp.Compare(a.Name, b.Name)
				})
				mode = runtime.AttrIndexSortedBinary
			default:
				mode = runtime.AttrIndexHash
				table := buildAttrHashTable(uses, off)
				hashTable = uint32(b.rt.AttributeHashTableCount())
				if _, err := b.assembler.AppendAttrHashTable(table); err != nil {
					return err
				}
			}
			for _, use := range uses {
				if _, err := b.assembler.AppendAttrUse(use); err != nil {
					return err
				}
			}
			complexType.Attrs = runtime.AttrIndexRef{
				Off:       off,
				Len:       uint32(len(uses)),
				Mode:      mode,
				HashTable: hashTable,
			}
		}
		if err := b.assembler.SetComplexType(complexID, complexType); err != nil {
			return err
		}
	}
	return nil
}
