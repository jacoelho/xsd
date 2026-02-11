package attrgroupwalk

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type MissingPolicy uint8

const (
	MissingIgnore MissingPolicy = iota
	MissingError
)

type CyclePolicy uint8

const (
	CycleIgnore CyclePolicy = iota
	CycleError
)

type Options struct {
	Missing MissingPolicy
	Cycles  CyclePolicy
}

type ErrMissing struct {
	QName model.QName
}

func (e ErrMissing) Error() string {
	return fmt.Sprintf("attributeGroup %s not found", e.QName)
}

type ErrCycle struct {
	QName model.QName
}

func (e ErrCycle) Error() string {
	return fmt.Sprintf("attributeGroup cycle detected at %s", e.QName)
}

type walkState uint8

const (
	walkStateVisiting walkState = iota + 1
	walkStateDone
)

type closureResult struct {
	order []model.QName
	err   error
}

// Context memoizes attribute-group closure traversal for repeated passes.
type Context struct {
	schema *parser.Schema
	opts   Options
	cache  map[model.QName]closureResult
}

// NewContext creates a traversal context that can be reused across passes.
func NewContext(schema *parser.Schema, opts Options) *Context {
	return &Context{
		schema: schema,
		opts:   opts,
		cache:  make(map[model.QName]closureResult),
	}
}

func Walk(schema *parser.Schema, refs []model.QName, missing MissingPolicy, visit func(model.QName, *model.AttributeGroup) error) error {
	return WalkWithOptions(schema, refs, Options{Missing: missing, Cycles: CycleIgnore}, visit)
}

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
			return nil, ErrCycle{QName: ref}
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
			return nil, ErrMissing{QName: ref}
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
