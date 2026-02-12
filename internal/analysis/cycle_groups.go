package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/graphcycle"
	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
)

func detectGroupCycles(schema *parser.Schema) error {
	starts := make([]model.QName, 0, len(schema.Groups))
	if err := globaldecl.ForEachGroup(schema, func(name model.QName, group *model.ModelGroup) error {
		if group == nil {
			return fmt.Errorf("missing group %s", name)
		}
		starts = append(starts, name)
		return nil
	}); err != nil {
		return err
	}

	err := graphcycle.Detect(graphcycle.Config[model.QName]{
		Starts:  starts,
		Missing: graphcycle.MissingPolicyError,
		Exists: func(name model.QName) bool {
			return schema.Groups[name] != nil
		},
		Next: func(name model.QName) ([]model.QName, error) {
			group := schema.Groups[name]
			if group == nil {
				return nil, nil
			}
			refs := collectGroupRefs(group)
			out := make([]model.QName, 0, len(refs))
			for _, ref := range refs {
				out = append(out, ref.RefQName)
			}
			return out, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr graphcycle.CycleError[model.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("group cycle detected at %s", cycleErr.Key)
	}
	var missingErr graphcycle.MissingError[model.QName]
	if errors.As(err, &missingErr) {
		return fmt.Errorf("group %s ref %s not found", missingErr.From, missingErr.Key)
	}
	return err
}

func collectGroupRefs(group *model.ModelGroup) []*model.GroupRef {
	if group == nil {
		return nil
	}
	var refs []*model.GroupRef
	for _, particle := range group.Particles {
		refs = collectGroupRefsFromParticle(particle, refs)
	}
	return refs
}

func collectGroupRefsFromParticle(particle model.Particle, refs []*model.GroupRef) []*model.GroupRef {
	switch typed := particle.(type) {
	case *model.GroupRef:
		return append(refs, typed)
	case *model.ModelGroup:
		for _, child := range typed.Particles {
			refs = collectGroupRefsFromParticle(child, refs)
		}
	case *model.ElementDecl, *model.AnyElement:
		return refs
	}
	return refs
}
