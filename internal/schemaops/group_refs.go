package schemaops

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

// GroupRefLookupFunc resolves a group reference to a model group definition.
type GroupRefLookupFunc func(ref *model.GroupRef) *model.ModelGroup

// GroupRefErrorFunc builds an expansion error for a referenced group QName.
type GroupRefErrorFunc func(ref model.QName) error

// AllGroupMode controls how xs:all groups are represented in expanded trees.
type AllGroupMode uint8

const (
	// AllGroupKeep preserves xs:all as an all-model group.
	AllGroupKeep AllGroupMode = iota
	// AllGroupAsChoice rewrites xs:all to choice for deterministic checks.
	AllGroupAsChoice
)

// LeafCloneMode controls whether leaf particles are copied or reused.
type LeafCloneMode uint8

const (
	// LeafReuse keeps leaf particle pointers unchanged.
	LeafReuse LeafCloneMode = iota
	// LeafClone copies leaf particle structs.
	LeafClone
)

// ExpandGroupRefsOptions configures group-reference expansion behavior.
type ExpandGroupRefsOptions struct {
	Lookup       GroupRefLookupFunc
	MissingError GroupRefErrorFunc
	CycleError   GroupRefErrorFunc
	AllGroupMode AllGroupMode
	LeafClone    LeafCloneMode
}

// ExpandGroupRefs expands group references and returns an equivalent particle tree.
func ExpandGroupRefs(particle model.Particle, opts ExpandGroupRefsOptions) (model.Particle, error) {
	cfg := opts.withDefaults()
	return expandGroupRefs(particle, cfg, make(map[model.QName]bool))
}

func (opts ExpandGroupRefsOptions) withDefaults() ExpandGroupRefsOptions {
	cfg := opts
	if cfg.Lookup == nil {
		cfg.Lookup = func(_ *model.GroupRef) *model.ModelGroup { return nil }
	}
	if cfg.MissingError == nil {
		cfg.MissingError = func(ref model.QName) error {
			return fmt.Errorf("group reference %s not found", ref)
		}
	}
	if cfg.CycleError == nil {
		cfg.CycleError = func(ref model.QName) error {
			return fmt.Errorf("group reference cycle detected: %s", ref)
		}
	}
	return cfg
}

func expandGroupRefs(
	particle model.Particle,
	opts ExpandGroupRefsOptions,
	stack map[model.QName]bool,
) (model.Particle, error) {
	switch typed := particle.(type) {
	case *model.GroupRef:
		if typed == nil {
			return nil, nil
		}
		if stack[typed.RefQName] {
			return nil, opts.CycleError(typed.RefQName)
		}
		group := opts.Lookup(typed)
		if group == nil {
			return nil, opts.MissingError(typed.RefQName)
		}

		stack[typed.RefQName] = true
		defer delete(stack, typed.RefQName)

		expanded, err := cloneExpandedModelGroup(group, opts, stack)
		if err != nil {
			return nil, err
		}
		expanded.MinOccurs = typed.MinOccurs
		expanded.MaxOccurs = typed.MaxOccurs
		return expanded, nil
	case *model.ModelGroup:
		if typed == nil {
			return nil, nil
		}
		return cloneExpandedModelGroup(typed, opts, stack)
	case *model.ElementDecl:
		if opts.LeafClone != LeafClone || typed == nil {
			return typed, nil
		}
		clone := *typed
		return &clone, nil
	case *model.AnyElement:
		if opts.LeafClone != LeafClone || typed == nil {
			return typed, nil
		}
		clone := *typed
		return &clone, nil
	default:
		return particle, nil
	}
}

func cloneExpandedModelGroup(
	group *model.ModelGroup,
	opts ExpandGroupRefsOptions,
	stack map[model.QName]bool,
) (*model.ModelGroup, error) {
	clone := &model.ModelGroup{
		Kind:      normalizeGroupKind(group.Kind, opts.AllGroupMode),
		MinOccurs: group.MinOccurs,
		MaxOccurs: group.MaxOccurs,
		Particles: make([]model.Particle, 0, len(group.Particles)),
	}
	for _, child := range group.Particles {
		expanded, err := expandGroupRefs(child, opts, stack)
		if err != nil {
			return nil, err
		}
		clone.Particles = append(clone.Particles, expanded)
	}
	return clone, nil
}

func normalizeGroupKind(kind model.GroupKind, mode AllGroupMode) model.GroupKind {
	if kind == model.AllGroup && mode == AllGroupAsChoice {
		return model.Choice
	}
	return kind
}
