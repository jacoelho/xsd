package semantic

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

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
