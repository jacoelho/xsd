package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) buildElements() error {
	if b.schema == nil {
		return fmt.Errorf("runtime build: schema ir is nil")
	}
	for _, entry := range b.schema.Elements {
		id := runtime.ElemID(entry.ID)
		if id == 0 {
			return fmt.Errorf("runtime build: element %s missing runtime ID", formatIRName(entry.Name))
		}
		sym, err := b.lookupIRSymbol(entry.Name)
		if err != nil {
			return err
		}
		elem := runtime.Element{Name: sym}
		typeID, ok := b.runtimeTypeIDFromIRRef(entry.TypeDecl)
		if !ok {
			return fmt.Errorf("runtime build: element %s missing type ID", formatIRName(entry.Name))
		}
		elem.Type = typeID
		if entry.SubstitutionHead != 0 {
			headID := runtime.ElemID(entry.SubstitutionHead)
			if headID == 0 {
				return fmt.Errorf("runtime build: element %s missing substitution head ID", formatIRName(entry.Name))
			}
			elem.SubstHead = headID
		}
		if entry.Nillable {
			elem.Flags |= runtime.ElemNillable
		}
		if entry.Abstract {
			elem.Flags |= runtime.ElemAbstract
		}
		elem.Block = toRuntimeIRElemBlock(entry.Block)
		elem.Final = toRuntimeIRDerivation(entry.Final)

		if def, ok := b.artifacts.ElementDefaults[entry.ID]; ok {
			elem.Default = def.Ref
			elem.DefaultKey = def.Key
			elem.DefaultMember = def.Member
		}
		if fixed, ok := b.artifacts.ElementFixed[entry.ID]; ok {
			elem.Fixed = fixed.Ref
			elem.FixedKey = fixed.Key
			elem.FixedMember = fixed.Member
		}

		if err := b.assembler.SetElement(id, elem); err != nil {
			return err
		}
		if entry.Global {
			if err := b.assembler.SetGlobalElement(sym, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func toRuntimeIRElemBlock(value schemair.ElementBlock) runtime.ElemBlock {
	var out runtime.ElemBlock
	if value&schemair.ElementBlockSubstitution != 0 {
		out |= runtime.ElemBlockSubstitution
	}
	if value&schemair.ElementBlockExtension != 0 {
		out |= runtime.ElemBlockExtension
	}
	if value&schemair.ElementBlockRestriction != 0 {
		out |= runtime.ElemBlockRestriction
	}
	return out
}

func (b *schemaBuilder) buildModels() error {
	if err := b.buildAnyTypeModel(); err != nil {
		return err
	}
	for _, plan := range b.schema.ComplexTypes {
		typeID := b.userTypeRuntimeID(plan.TypeDecl)
		complexID := b.complexIDs[typeID]
		if complexID == 0 {
			continue
		}

		complexModel, ok := b.rt.ComplexType(complexID)
		if !ok {
			return fmt.Errorf("runtime build: complex type %d out of range", complexID)
		}
		complexModel.Mixed = plan.Mixed
		switch plan.Content {
		case schemair.ContentSimple:
			complexModel.Content = runtime.ContentSimple
			vid, ok := b.validatorForIRTypeRef(plan.TextType)
			if !ok || vid == 0 {
				vid = b.artifacts.TextValidators[plan.TypeDecl]
			}
			if vid == 0 {
				return fmt.Errorf("runtime build: complex type %d missing text validator", plan.TypeDecl)
			}
			complexModel.TextValidator = vid
		case schemair.ContentEmpty:
			complexModel.Content = runtime.ContentEmpty
		default:
			if plan.Particle == 0 {
				complexModel.Content = runtime.ContentEmpty
				break
			}
			ref, kind, err := b.compileParticleModel(plan.Particle)
			if err != nil {
				return err
			}
			complexModel.Content = kind
			complexModel.Model = ref
		}
		if err := b.assembler.SetComplexType(complexID, complexModel); err != nil {
			return err
		}
	}
	return nil
}

func (b *schemaBuilder) buildAnyTypeModel() error {
	complexModel, ok := b.rt.ComplexType(b.anyTypeComplex)
	if !ok {
		return nil
	}
	complexModel.Mixed = true

	wildcard := b.addWildcardRule(schemair.Wildcard{
		NamespaceKind:   schemair.NamespaceAny,
		ProcessContents: schemair.ProcessLax,
	})
	ref, kind, err := b.compileParticleTree(contentmodel.TreeParticle{
		Kind:        contentmodel.TreeWildcard,
		WildcardID:  uint32(wildcard),
		Min:         contentmodel.TreeOccurs{Value: 0},
		Max:         contentmodel.TreeOccurs{Unbounded: true},
		RuntimeRule: true,
	})
	if err != nil {
		return err
	}
	complexModel.Content = kind
	complexModel.Model = ref

	complexModel.AnyAttr = b.addWildcardRule(schemair.Wildcard{
		NamespaceKind:   schemair.NamespaceAny,
		ProcessContents: schemair.ProcessLax,
	})
	return b.assembler.SetComplexType(b.anyTypeComplex, complexModel)
}
