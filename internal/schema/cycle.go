package schema

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type visitState uint8

const (
	stateUnseen visitState = iota
	stateVisiting
	stateDone
)

// DetectCycles validates that type derivation, group refs, attribute group refs,
// and substitution groups are acyclic.
func DetectCycles(schema *parser.Schema) error {
	if err := RequireResolved(schema); err != nil {
		return err
	}
	if err := validateSchemaInput(schema); err != nil {
		return err
	}

	if err := detectTypeCycles(schema); err != nil {
		return err
	}
	if err := detectGroupCycles(schema); err != nil {
		return err
	}
	if err := detectAttributeGroupCycles(schema); err != nil {
		return err
	}
	if err := detectSubstitutionGroupCycles(schema); err != nil {
		return err
	}
	return nil
}

func detectTypeCycles(schema *parser.Schema) error {
	states := make(map[types.QName]visitState)

	var visit func(name types.QName, typ types.Type) error
	visit = func(name types.QName, typ types.Type) error {
		if name.IsZero() {
			return nil
		}
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("type cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		base := typeBaseQName(typ)
		if !base.IsZero() && base.Namespace != types.XSDNamespace {
			baseType := schema.TypeDefs[base]
			if baseType == nil {
				return fmt.Errorf("type %s base %s not found", name, base)
			}
			if err := visit(base, baseType); err != nil {
				return err
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclType {
			continue
		}
		typ := schema.TypeDefs[decl.Name]
		if typ == nil {
			return fmt.Errorf("missing global type %s", decl.Name)
		}
		if err := visit(decl.Name, typ); err != nil {
			return err
		}
	}
	return nil
}

func typeBaseQName(typ types.Type) types.QName {
	switch typed := typ.(type) {
	case *types.SimpleType:
		if typed.Restriction == nil {
			return types.QName{}
		}
		return typed.Restriction.Base
	case *types.ComplexType:
		if typed.Content() == nil {
			return types.QName{}
		}
		return typed.Content().BaseTypeQName()
	default:
		return types.QName{}
	}
}

func detectGroupCycles(schema *parser.Schema) error {
	states := make(map[types.QName]visitState)

	var visit func(name types.QName, group *types.ModelGroup) error
	visit = func(name types.QName, group *types.ModelGroup) error {
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("group cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		for _, ref := range collectGroupRefs(group) {
			target := schema.Groups[ref.RefQName]
			if target == nil {
				return fmt.Errorf("group %s ref %s not found", name, ref.RefQName)
			}
			if err := visit(ref.RefQName, target); err != nil {
				return err
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclGroup {
			continue
		}
		group := schema.Groups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing group %s", decl.Name)
		}
		if err := visit(decl.Name, group); err != nil {
			return err
		}
	}
	return nil
}

func collectGroupRefs(group *types.ModelGroup) []*types.GroupRef {
	if group == nil {
		return nil
	}
	var refs []*types.GroupRef
	for _, particle := range group.Particles {
		refs = collectGroupRefsFromParticle(particle, refs)
	}
	return refs
}

func collectGroupRefsFromParticle(particle types.Particle, refs []*types.GroupRef) []*types.GroupRef {
	switch typed := particle.(type) {
	case *types.GroupRef:
		return append(refs, typed)
	case *types.ModelGroup:
		for _, child := range typed.Particles {
			refs = collectGroupRefsFromParticle(child, refs)
		}
	case *types.ElementDecl, *types.AnyElement:
		return refs
	}
	return refs
}

func detectAttributeGroupCycles(schema *parser.Schema) error {
	states := make(map[types.QName]visitState)

	var visit func(name types.QName, group *types.AttributeGroup) error
	visit = func(name types.QName, group *types.AttributeGroup) error {
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("attributeGroup cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		for _, ref := range group.AttrGroups {
			target := schema.AttributeGroups[ref]
			if target == nil {
				return fmt.Errorf("attributeGroup %s ref %s not found", name, ref)
			}
			if err := visit(ref, target); err != nil {
				return err
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclAttributeGroup {
			continue
		}
		group := schema.AttributeGroups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", decl.Name)
		}
		if err := visit(decl.Name, group); err != nil {
			return err
		}
	}
	return nil
}

func detectSubstitutionGroupCycles(schema *parser.Schema) error {
	states := make(map[types.QName]visitState)

	var visit func(name types.QName) error
	visit = func(name types.QName) error {
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("substitution group cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		decl := schema.ElementDecls[name]
		if decl == nil {
			return fmt.Errorf("element %s not found", name)
		}
		if !decl.SubstitutionGroup.IsZero() {
			head := decl.SubstitutionGroup
			if _, ok := schema.ElementDecls[head]; !ok {
				return fmt.Errorf("element %s substitutionGroup %s not found", name, head)
			}
			if err := visit(head); err != nil {
				return err
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclElement {
			continue
		}
		if err := visit(decl.Name); err != nil {
			return err
		}
	}
	return nil
}
