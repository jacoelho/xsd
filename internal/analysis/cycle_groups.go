package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/graphcycle"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func detectGroupCycles(schema *parser.Schema) error {
	starts := make([]types.QName, 0, len(schema.Groups))
	if err := globaldecl.ForEachGroup(schema, func(name types.QName, group *types.ModelGroup) error {
		if group == nil {
			return fmt.Errorf("missing group %s", name)
		}
		starts = append(starts, name)
		return nil
	}); err != nil {
		return err
	}

	err := graphcycle.Detect(graphcycle.Config[types.QName]{
		Starts:  starts,
		Missing: graphcycle.MissingPolicyError,
		Exists: func(name types.QName) bool {
			return schema.Groups[name] != nil
		},
		Next: func(name types.QName) ([]types.QName, error) {
			group := schema.Groups[name]
			if group == nil {
				return nil, nil
			}
			refs := collectGroupRefs(group)
			out := make([]types.QName, 0, len(refs))
			for _, ref := range refs {
				out = append(out, ref.RefQName)
			}
			return out, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr graphcycle.CycleError[types.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("group cycle detected at %s", cycleErr.Key)
	}
	var missingErr graphcycle.MissingError[types.QName]
	if errors.As(err, &missingErr) {
		return fmt.Errorf("group %s ref %s not found", missingErr.From, missingErr.Key)
	}
	return err
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
