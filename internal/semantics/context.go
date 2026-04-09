package semantics

import (
	"fmt"
	"sync"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Context stores prepared compile-time semantic state for one resolved schema graph.
type Context struct {
	schema   *parser.Schema
	registry *analysis.Registry
	refs     *analysis.ResolvedReferences

	complex      *complexplan.ComplexTypes
	particles    *Particles
	simpleTypes  *SimpleTypes
	substitution *Substitution

	complexErr error

	complexOnce      sync.Once
	particlesOnce    sync.Once
	simpleTypesOnce  sync.Once
	substitutionOnce sync.Once
}

// Build creates a compile-time semantics context for a prepared schema graph.
func Build(schema *parser.Schema, registry *analysis.Registry, refs *analysis.ResolvedReferences) (*Context, error) {
	if schema == nil {
		return nil, fmt.Errorf("semantics: schema is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("semantics: registry is nil")
	}
	if refs == nil {
		return nil, fmt.Errorf("semantics: references are nil")
	}
	return &Context{
		schema:   schema,
		registry: registry,
		refs:     refs,
	}, nil
}

// Schema returns the prepared schema graph.
func (c *Context) Schema() *parser.Schema { return c.schema }

// Registry returns the deterministic registry for the prepared schema graph.
func (c *Context) Registry() *analysis.Registry { return c.registry }

// References returns resolved runtime references for the prepared schema graph.
func (c *Context) References() *analysis.ResolvedReferences { return c.refs }

// ComplexTypes returns effective complex-type semantics for the prepared schema graph.
func (c *Context) ComplexTypes() (*complexplan.ComplexTypes, error) {
	if c == nil {
		return nil, fmt.Errorf("semantics: context is nil")
	}
	c.complexOnce.Do(func() {
		complexTypes, err := buildComplexTypes(c.schema, c.registry, nil)
		if err != nil {
			c.complexErr = err
			return
		}
		c.complex = complexTypes
	})
	if c.complexErr != nil {
		return nil, c.complexErr
	}
	return c.complex, nil
}

// Particles returns particle preparation and validation
func (c *Context) Particles() *Particles {
	if c == nil {
		return nil
	}
	c.particlesOnce.Do(func() {
		c.particles = &Particles{ctx: c}
	})
	return c.particles
}

// SimpleTypes returns the simple-type semantic view.
func (c *Context) SimpleTypes() *SimpleTypes {
	if c == nil {
		return nil
	}
	c.simpleTypesOnce.Do(func() {
		c.simpleTypes = &SimpleTypes{ctx: c}
	})
	return c.simpleTypes
}

// Substitution returns substitution-group semantic view.
func (c *Context) Substitution() *Substitution {
	if c == nil {
		return nil
	}
	c.substitutionOnce.Do(func() {
		c.substitution = &Substitution{ctx: c}
	})
	return c.substitution
}

// Particles exposes particle preparation and validation
type Particles struct {
	ctx *Context
}

// ValidateUPA validates UPA against the prepared schema graph.
func (p *Particles) ValidateUPA() error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("semantics: particles context is nil")
	}
	for _, entry := range p.ctx.registry.TypeOrder {
		ct, ok := entry.Type.(*model.ComplexType)
		if !ok {
			continue
		}
		if err := ValidateUPA(p.ctx.schema, ct.Content(), p.ctx.schema.TargetNamespace); err != nil {
			if ct == nil || ct.QName.IsZero() {
				return fmt.Errorf("anonymous complexType: %w", err)
			}
			return fmt.Errorf("complexType %s: %w", ct.QName, err)
		}
	}
	return nil
}

// SimpleTypes exposes simple-type semantic compilation.
type SimpleTypes struct {
	ctx *Context
}

// ValidateDefaultOrFixed validates a default/fixed value against the prepared
// schema graph owned by the context.
func (s *SimpleTypes) ValidateDefaultOrFixed(value string, typ model.Type, context map[string]string, policy IDPolicy) error {
	if s == nil || s.ctx == nil {
		return fmt.Errorf("semantics: simple types context is nil")
	}
	return ValidateDefaultOrFixedResolved(s.ctx.schema, value, typ, context, policy)
}

// ValidateWithFacets validates a lexical value using schema-time facet
// conversion against the prepared schema graph owned by the context.
func (s *SimpleTypes) ValidateWithFacets(value string, typ model.Type, context map[string]string, convert model.DeferredFacetConverter) error {
	if s == nil || s.ctx == nil {
		return fmt.Errorf("semantics: simple types context is nil")
	}
	return ValidateWithFacets(s.ctx.schema, value, typ, context, convert)
}

// Substitution exposes substitution-group semantic compilation.
type Substitution struct {
	ctx *Context
}
