package runtimeassemble

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (b *schemaBuilder) buildAttributes() error {
	if err := b.buildGlobalAttributes(); err != nil {
		return err
	}
	if err := b.buildComplexTypeAttributeIndexes(); err != nil {
		return err
	}
	return nil
}

func (b *schemaBuilder) buildGlobalAttributes() error {
	for _, entry := range b.registry.AttributeOrder {
		id := b.attrIDs[entry.ID]
		sym := b.internQName(entry.QName)
		attr := runtime.Attribute{Name: sym}
		if decl := entry.Decl; decl != nil {
			if decl.Type == nil {
				return fmt.Errorf("runtime build: attribute %s missing type", entry.QName)
			}
			vid, ok := b.validators.ValidatorForType(decl.Type)
			if !ok || vid == 0 {
				return fmt.Errorf("runtime build: attribute %s missing validator", entry.QName)
			}
			attr.Validator = vid
		}
		if def, ok := b.validators.AttributeDefault(entry.ID); ok {
			attr.Default = def.Ref
			attr.DefaultKey = def.Key
			attr.DefaultMember = def.Member
		}
		if fixed, ok := b.validators.AttributeFixed(entry.ID); ok {
			attr.Fixed = fixed.Ref
			attr.FixedKey = fixed.Key
			attr.FixedMember = fixed.Member
		}
		b.rt.Attributes[id] = attr
		if entry.Global {
			b.rt.GlobalAttributes[sym] = id
		}
	}
	return nil
}

func (b *schemaBuilder) buildComplexTypeAttributeIndexes() error {
	for _, entry := range b.registry.TypeOrder {
		ct, ok := model.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		typeID := b.typeIDs[entry.ID]
		complexID := b.complexIDs[typeID]
		if complexID == 0 {
			continue
		}

		uses, wildcard, err := b.collectAttrUses(ct)
		if err != nil {
			return err
		}
		if wildcard != nil {
			b.rt.ComplexTypes[complexID].AnyAttr = b.addWildcard(
				wildcard.Namespace,
				wildcard.NamespaceList,
				wildcard.TargetNamespace,
				wildcard.ProcessContents,
			)
		}
		if len(uses) == 0 {
			continue
		}
		off := uint32(len(b.rt.AttrIndex.Uses))
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
			hashTable = uint32(len(b.rt.AttrIndex.HashTables))
			b.rt.AttrIndex.HashTables = append(b.rt.AttrIndex.HashTables, table)
		}
		b.rt.AttrIndex.Uses = append(b.rt.AttrIndex.Uses, uses...)
		ref := runtime.AttrIndexRef{
			Off:       off,
			Len:       uint32(len(uses)),
			Mode:      mode,
			HashTable: hashTable,
		}
		b.rt.ComplexTypes[complexID].Attrs = ref
	}
	return nil
}
