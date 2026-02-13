package attrgroupwalk

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

// Options configures attribute-group traversal behavior.
type Options struct {
	Missing MissingPolicy
	Cycles  CyclePolicy
}

// AttrGroupMissingError reports a missing attributeGroup reference.
type AttrGroupMissingError struct {
	QName model.QName
}

// Error returns the formatted error message.
func (e AttrGroupMissingError) Error() string {
	return fmt.Sprintf("attributeGroup %s not found", e.QName)
}

// AttrGroupCycleError reports a cycle in attributeGroup references.
type AttrGroupCycleError struct {
	QName model.QName
}

// Error returns the formatted error message.
func (e AttrGroupCycleError) Error() string {
	return fmt.Sprintf("attributeGroup cycle detected at %s", e.QName)
}

type walkState uint8

const (
	walkStateVisiting walkState = iota + 1
	walkStateDone
)

type closureResult struct {
	err   error
	order []model.QName
}

// Context memoizes attribute-group closure traversal for repeated passes.
type Context struct {
	schema *parser.Schema
	cache  map[model.QName]closureResult
	opts   Options
}

// NewContext creates a traversal context that can be reused across passes.
func NewContext(schema *parser.Schema, opts Options) *Context {
	return &Context{
		schema: schema,
		opts:   opts,
		cache:  make(map[model.QName]closureResult),
	}
}

// Walk traverses referenced attribute groups using the default cycle policy.
func Walk(schema *parser.Schema, refs []model.QName, missing MissingPolicy, visit func(model.QName, *model.AttributeGroup) error) error {
	return WalkWithOptions(schema, refs, Options{Missing: missing, Cycles: CycleIgnore}, visit)
}

// WalkWithOptions traverses referenced attribute groups using explicit options.
func WalkWithOptions(schema *parser.Schema, refs []model.QName, opts Options, visit func(model.QName, *model.AttributeGroup) error) error {
	return NewContext(schema, opts).Walk(refs, visit)
}

// Walk visits attribute groups in deterministic order with per-call deduplication.
func (c *Context) Walk(refs []model.QName, visit func(model.QName, *model.AttributeGroup) error) error {
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

func (c *Context) closureFor(ref model.QName) ([]model.QName, error) {
	if cached, ok := c.cache[ref]; ok {
		return cached.order, cached.err
	}

	state := make(map[model.QName]walkState)
	order, err := c.walkClosure(ref, state)
	c.cache[ref] = closureResult{order: order, err: err}
	return order, err
}

func (c *Context) walkClosure(ref model.QName, state map[model.QName]walkState) ([]model.QName, error) {
	switch state[ref] {
	case walkStateDone:
		return nil, nil
	case walkStateVisiting:
		if c.opts.Cycles == CycleError {
			return nil, AttrGroupCycleError{QName: ref}
		}
		return nil, nil
	}
	state[ref] = walkStateVisiting
	defer func() {
		state[ref] = walkStateDone
	}()

	group, ok := c.schema.AttributeGroups[ref]
	if !ok || group == nil {
		if c.opts.Missing == MissingError {
			return nil, AttrGroupMissingError{QName: ref}
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
