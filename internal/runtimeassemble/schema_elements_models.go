package runtimeassemble

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/typechain"
)

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

		if def, ok := b.validators.ElementDefault(entry.ID); ok {
			elem.Default = def.Ref
			elem.DefaultKey = def.Key
			elem.DefaultMember = def.Member
		}
		if fixed, ok := b.validators.ElementFixed(entry.ID); ok {
			elem.Fixed = fixed.Ref
			elem.FixedKey = fixed.Key
			elem.FixedMember = fixed.Member
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
		ct, ok := model.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		typeID := b.typeIDs[entry.ID]
		complexID := b.complexIDs[typeID]
		if complexID == 0 {
			continue
		}

		content := ct.Content()
		complexModel := &b.rt.ComplexTypes[complexID]
		complexModel.Mixed = ct.EffectiveMixed()
		switch content.(type) {
		case *model.SimpleContent:
			complexModel.Content = runtime.ContentSimple
			var textType model.Type
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
			complexModel.TextValidator = vid
		case *model.EmptyContent:
			complexModel.Content = runtime.ContentEmpty
		default:
			particle := typechain.EffectiveContentParticle(b.schema, ct)
			if particle == nil {
				complexModel.Content = runtime.ContentEmpty
				break
			}
			ref, kind, err := b.compileParticleModel(particle)
			if err != nil {
				return err
			}
			complexModel.Content = kind
			complexModel.Model = ref
		}

	}
	return nil
}

func (b *schemaBuilder) buildAnyTypeModel() error {
	if b.anyTypeComplex == 0 || int(b.anyTypeComplex) >= len(b.rt.ComplexTypes) {
		return nil
	}
	complexModel := &b.rt.ComplexTypes[b.anyTypeComplex]
	complexModel.Mixed = true

	anyElem := &model.AnyElement{
		Namespace:       model.NSCAny,
		ProcessContents: model.Lax,
		MinOccurs:       model.OccursFromInt(0),
		MaxOccurs:       model.OccursUnbounded,
	}
	ref, kind, err := b.compileParticleModel(anyElem)
	if err != nil {
		return err
	}
	complexModel.Content = kind
	complexModel.Model = ref

	anyAttr := &model.AnyAttribute{
		Namespace:       model.NSCAny,
		ProcessContents: model.Lax,
		TargetNamespace: model.NamespaceEmpty,
	}
	complexModel.AnyAttr = b.addWildcard(
		anyAttr.Namespace,
		anyAttr.NamespaceList,
		anyAttr.TargetNamespace,
		anyAttr.ProcessContents,
	)
	return nil
}
