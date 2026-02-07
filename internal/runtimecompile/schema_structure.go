package runtimecompile

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/typegraph"
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

func (b *schemaBuilder) buildElements() error {
	for _, entry := range b.registry.ElementOrder {
		id := b.elemIDs[entry.ID]
		decl := entry.Decl
		if decl == nil {
			return fmt.Errorf("runtime build: element %s is nil", entry.QName)
		}
		sym := b.internQName(entry.QName)
		elem := runtime.Element{Name: sym}
		if decl.Type == nil {
			return fmt.Errorf("runtime build: element %s missing type", entry.QName)
		}
		typeID, ok := b.runtimeTypeID(decl.Type)
		if !ok {
			return fmt.Errorf("runtime build: element %s missing type ID", entry.QName)
		}
		elem.Type = typeID
		if !decl.SubstitutionGroup.IsZero() {
			if head := b.schema.ElementDecls[decl.SubstitutionGroup]; head != nil {
				if headID, ok := b.runtimeElemID(head); ok {
					elem.SubstHead = headID
				}
			}
		}
		if decl.Nillable {
			elem.Flags |= runtime.ElemNillable
		}
		if decl.Abstract {
			elem.Flags |= runtime.ElemAbstract
		}
		elem.Block = toRuntimeElemBlock(decl.Block)
		elem.Final = toRuntimeDerivationSet(decl.Final)

		if def, ok := b.validators.ElementDefaults[entry.ID]; ok {
			elem.Default = def
			if key, ok := b.validators.ElementDefaultKeys[entry.ID]; ok {
				elem.DefaultKey = key
			}
			if member, ok := b.validators.ElementDefaultMembers[entry.ID]; ok {
				elem.DefaultMember = member
			}
		}
		if fixed, ok := b.validators.ElementFixed[entry.ID]; ok {
			elem.Fixed = fixed
			if key, ok := b.validators.ElementFixedKeys[entry.ID]; ok {
				elem.FixedKey = key
			}
			if member, ok := b.validators.ElementFixedMembers[entry.ID]; ok {
				elem.FixedMember = member
			}
		}

		b.rt.Elements[id] = elem
		if entry.Global {
			b.rt.GlobalElements[sym] = id
		}
	}
	return nil
}

func (b *schemaBuilder) buildModels() error {
	if err := b.buildAnyTypeModel(); err != nil {
		return err
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

		content := ct.Content()
		model := &b.rt.ComplexTypes[complexID]
		model.Mixed = ct.EffectiveMixed()
		switch content.(type) {
		case *types.SimpleContent:
			model.Content = runtime.ContentSimple
			var textType types.Type
			if b.validators != nil && b.validators.SimpleContentTypes != nil {
				textType = b.validators.SimpleContentTypes[ct]
			}
			if textType == nil {
				var err error
				textType, err = b.simpleContentTextType(ct)
				if err != nil {
					return err
				}
			}
			if textType == nil {
				return fmt.Errorf("runtime build: complex type %s simpleContent base missing", entry.QName)
			}
			vid, ok := b.validators.ValidatorForType(textType)
			if !ok || vid == 0 {
				return fmt.Errorf("runtime build: complex type %s missing validator", entry.QName)
			}
			model.TextValidator = vid
		case *types.EmptyContent:
			model.Content = runtime.ContentEmpty
		default:
			particle := typegraph.EffectiveContentParticle(b.schema, ct)
			if particle == nil {
				model.Content = runtime.ContentEmpty
				break
			}
			ref, kind, err := b.compileParticleModel(particle)
			if err != nil {
				return err
			}
			model.Content = kind
			model.Model = ref
		}

	}
	return nil
}

func (b *schemaBuilder) buildAnyTypeModel() error {
	if b.anyTypeComplex == 0 || int(b.anyTypeComplex) >= len(b.rt.ComplexTypes) {
		return nil
	}
	model := &b.rt.ComplexTypes[b.anyTypeComplex]
	model.Mixed = true

	anyElem := &types.AnyElement{
		Namespace:       types.NSCAny,
		ProcessContents: types.Lax,
		MinOccurs:       types.OccursFromInt(0),
		MaxOccurs:       types.OccursUnbounded,
	}
	ref, kind, err := b.compileParticleModel(anyElem)
	if err != nil {
		return err
	}
	model.Content = kind
	model.Model = ref

	anyAttr := &types.AnyAttribute{
		Namespace:       types.NSCAny,
		ProcessContents: types.Lax,
		TargetNamespace: types.NamespaceEmpty,
	}
	model.AnyAttr = b.addWildcardAnyAttribute(anyAttr)
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
