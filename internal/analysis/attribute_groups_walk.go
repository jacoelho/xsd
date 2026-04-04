package analysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// MissingPolicy controls behavior when a referenced attribute group is not found.
type MissingPolicy uint8

const (
	MissingIgnore MissingPolicy = iota
	MissingError
)

// CyclePolicy controls behavior when a traversal cycle is found.
type CyclePolicy uint8

const (
	CycleIgnore CyclePolicy = iota
	CycleError
)

// AttributeGroupWalkOptions configures attribute-group traversal behavior.
type AttributeGroupWalkOptions struct {
	Missing MissingPolicy
	Cycles  CyclePolicy
}

// AttributeGroupMissingError reports a missing attributeGroup reference.
type AttributeGroupMissingError struct {
	QName model.QName
}

func (e AttributeGroupMissingError) Error() string {
	return fmt.Sprintf("attributeGroup %s not found", e.QName)
}

// AttributeGroupCycleError reports a cycle in attributeGroup references.
type AttributeGroupCycleError struct {
	QName model.QName
}

func (e AttributeGroupCycleError) Error() string {
	return fmt.Sprintf("attributeGroup cycle detected at %s", e.QName)
}

type attributeGroupWalkState uint8

const (
	attributeGroupWalkStateVisiting attributeGroupWalkState = iota + 1
	attributeGroupWalkStateDone
)

type attributeGroupClosureResult struct {
	err   error
	order []model.QName
}

// AttributeGroupContext memoizes attribute-group closure traversal for repeated passes.
type AttributeGroupContext struct {
	schema *parser.Schema
	cache  map[model.QName]attributeGroupClosureResult
	opts   AttributeGroupWalkOptions
}

// NewAttributeGroupContext creates a traversal context that can be reused across passes.
func NewAttributeGroupContext(schema *parser.Schema, opts AttributeGroupWalkOptions) *AttributeGroupContext {
	return &AttributeGroupContext{
		schema: schema,
		opts:   opts,
		cache:  make(map[model.QName]attributeGroupClosureResult),
	}
}

// WalkAttributeGroups traverses referenced attribute groups using the default cycle policy.
func WalkAttributeGroups(
	schema *parser.Schema,
	refs []model.QName,
	missing MissingPolicy,
	visit func(model.QName, *model.AttributeGroup) error,
) error {
	return WalkAttributeGroupsWithOptions(schema, refs, AttributeGroupWalkOptions{
		Missing: missing,
		Cycles:  CycleIgnore,
	}, visit)
}

// WalkAttributeGroupsWithOptions traverses referenced attribute groups using explicit options.
func WalkAttributeGroupsWithOptions(
	schema *parser.Schema,
	refs []model.QName,
	opts AttributeGroupWalkOptions,
	visit func(model.QName, *model.AttributeGroup) error,
) error {
	return NewAttributeGroupContext(schema, opts).Walk(refs, visit)
}

// Walk visits attribute groups in deterministic order with per-call deduplication.
func (c *AttributeGroupContext) Walk(refs []model.QName, visit func(model.QName, *model.AttributeGroup) error) error {
	if c == nil || c.schema == nil || len(refs) == 0 {
		return nil
	}

	seen := make(map[model.QName]bool, len(refs))
	for _, ref := range refs {
		closure, err := c.closureFor(ref)
		if err != nil {
			return err
		}
		for _, name := range closure {
			if seen[name] {
				continue
			}
			seen[name] = true
			if visit == nil {
				continue
			}
			group := c.schema.AttributeGroups[name]
			if group == nil {
				continue
			}
			if err := visit(name, group); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *AttributeGroupContext) closureFor(ref model.QName) ([]model.QName, error) {
	if cached, ok := c.cache[ref]; ok {
		return cached.order, cached.err
	}

	state := make(map[model.QName]attributeGroupWalkState)
	order, err := c.walkClosure(ref, state)
	c.cache[ref] = attributeGroupClosureResult{order: order, err: err}
	return order, err
}

func (c *AttributeGroupContext) walkClosure(
	ref model.QName,
	state map[model.QName]attributeGroupWalkState,
) ([]model.QName, error) {
	switch state[ref] {
	case attributeGroupWalkStateDone:
		return nil, nil
	case attributeGroupWalkStateVisiting:
		if c.opts.Cycles == CycleError {
			return nil, AttributeGroupCycleError{QName: ref}
		}
		return nil, nil
	}
	state[ref] = attributeGroupWalkStateVisiting
	defer func() {
		state[ref] = attributeGroupWalkStateDone
	}()

	group, ok := c.schema.AttributeGroups[ref]
	if !ok || group == nil {
		if c.opts.Missing == MissingError {
			return nil, AttributeGroupMissingError{QName: ref}
		}
		return nil, nil
	}

	order := []model.QName{ref}
	for _, nested := range group.AttrGroups {
		nestedOrder, err := c.walkClosure(nested, state)
		if err != nil {
			return nil, err
		}
		order = append(order, nestedOrder...)
	}
	return order, nil
}
