package runtimecompile

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) buildAttributes() error {
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
		if def, ok := b.validators.AttributeDefaults[entry.ID]; ok {
			attr.Default = def
			if key, ok := b.validators.AttributeDefaultKeys[entry.ID]; ok {
				attr.DefaultKey = key
			}
			if member, ok := b.validators.AttributeDefaultMembers[entry.ID]; ok {
				attr.DefaultMember = member
			}
		}
		if fixed, ok := b.validators.AttributeFixed[entry.ID]; ok {
			attr.Fixed = fixed
			if key, ok := b.validators.AttributeFixedKeys[entry.ID]; ok {
				attr.FixedKey = key
			}
			if member, ok := b.validators.AttributeFixedMembers[entry.ID]; ok {
				attr.FixedMember = member
			}
		}
		b.rt.Attributes[id] = attr
		if entry.Global {
			b.rt.GlobalAttributes[sym] = id
		}
	}

	for _, entry := range b.registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
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
			b.rt.ComplexTypes[complexID].AnyAttr = b.addWildcardAnyAttribute(wildcard)
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
func (b *schemaBuilder) collectAttrUses(ct *types.ComplexType) ([]runtime.AttrUse, *types.AnyAttribute, error) {
	if ct == nil {
		return nil, nil, nil
	}
	attrs, wildcard, err := collectAttributeUses(b.schema, ct)
	if err != nil {
		return nil, nil, err
	}
	if len(attrs) == 0 {
		return nil, wildcard, nil
	}
	out := make([]runtime.AttrUse, 0, len(attrs))
	for _, decl := range attrs {
		if decl == nil {
			continue
		}
		target := decl
		if decl.IsReference {
			target = b.resolveAttributeDecl(decl)
			if target == nil {
				return nil, nil, fmt.Errorf("runtime build: attribute ref %s not found", decl.Name)
			}
		}
		sym := b.internQName(effectiveAttributeQName(b.schema, decl))
		use := runtime.AttrUse{
			Name: sym,
			Use:  toRuntimeAttrUse(decl.Use),
		}
		if target.Type == nil {
			return nil, nil, fmt.Errorf("runtime build: attribute %s missing type", target.Name)
		}
		vid, ok := b.validators.ValidatorForType(target.Type)
		if !ok || vid == 0 {
			return nil, nil, fmt.Errorf("runtime build: attribute %s missing validator", target.Name)
		}
		use.Validator = vid
		if decl.HasDefault {
			if def, ok := b.validators.AttrUseDefaults[decl]; ok {
				use.Default = def
				if key, ok := b.validators.AttrUseDefaultKeys[decl]; ok {
					use.DefaultKey = key
				}
				if member, ok := b.validators.AttrUseDefaultMembers[decl]; ok {
					use.DefaultMember = member
				}
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s default missing", decl.Name)
			}
		}
		if decl.HasFixed {
			if fixed, ok := b.validators.AttrUseFixed[decl]; ok {
				use.Fixed = fixed
				if key, ok := b.validators.AttrUseFixedKeys[decl]; ok {
					use.FixedKey = key
				}
				if member, ok := b.validators.AttrUseFixedMembers[decl]; ok {
					use.FixedMember = member
				}
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s fixed missing", decl.Name)
			}
		}
		if !use.Default.Present && !use.Fixed.Present {
			if attrID, ok := b.schemaAttrID(target); ok {
				if def, ok := b.validators.AttributeDefaults[attrID]; ok {
					use.Default = def
					if key, ok := b.validators.AttributeDefaultKeys[attrID]; ok {
						use.DefaultKey = key
					}
					if member, ok := b.validators.AttributeDefaultMembers[attrID]; ok {
						use.DefaultMember = member
					}
				}
				if fixed, ok := b.validators.AttributeFixed[attrID]; ok {
					use.Fixed = fixed
					if key, ok := b.validators.AttributeFixedKeys[attrID]; ok {
						use.FixedKey = key
					}
					if member, ok := b.validators.AttributeFixedMembers[attrID]; ok {
						use.FixedMember = member
					}
				}
			}
		}
		out = append(out, use)
	}
	return out, wildcard, nil
}

const (
	attrIndexLinearLimit = 8
	attrIndexBinaryLimit = 64
)

func buildAttrHashTable(uses []runtime.AttrUse, off uint32) runtime.AttrHashTable {
	size := max(runtime.NextPow2(len(uses)*2), 1)
	table := runtime.AttrHashTable{
		Hash: make([]uint64, size),
		Slot: make([]uint32, size),
	}
	mask := uint64(size - 1)
	for i := range uses {
		use := &uses[i]
		h := uint64(use.Name)
		if h == 0 {
			h = 1
		}
		slot := int(h & mask)
		for {
			if table.Slot[slot] == 0 {
				table.Hash[slot] = h
				table.Slot[slot] = off + uint32(i) + 1
				break
			}
			slot = (slot + 1) & int(mask)
		}
	}
	return table
}

func (b *schemaBuilder) resolveAttributeDecl(decl *types.AttributeDecl) *types.AttributeDecl {
	if decl == nil {
		return nil
	}
	if !decl.IsReference {
		return decl
	}
	return b.schema.AttributeDecls[decl.Name]
}

func (b *schemaBuilder) schemaAttrID(decl *types.AttributeDecl) (schema.AttrID, bool) {
	if decl == nil {
		return 0, false
	}
	if decl.IsReference {
		if id, ok := b.refs.AttributeRefs[decl]; ok {
			return id, true
		}
		return 0, false
	}
	if id, ok := b.registry.Attributes[decl.Name]; ok {
		return id, true
	}
	if id, ok := b.registry.LocalAttributes[decl]; ok {
		return id, true
	}
	return 0, false
}
