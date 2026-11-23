package schema

import "github.com/jacoelho/xsd/xsderrors"

// componentReach is the walk from the components an instance can name to the
// components validation can assess. An instance names only global
// declarations and types: a root element, xsi:type, a wildcard and a
// substitution group member all resolve by global name. From there validation
// reaches an element declaration's type, and a complex type's content model
// with every element particle in it and in its nested models. Each model is
// visited once by ID, so a group shared by many types is read once.
type componentReach struct {
	elements     []bool
	complexTypes []bool
	models       []bool
	queue        []contentModelID
}

// detachUnreachableComponents marks the components no instance element
// reaches. It is the one writer of complexTypeMetadata.Detached,
// identityConstraintSpec.Detached and schemaBuild.ReachableElements. An
// unreachable element declaration keeps its ID but owns no identity
// constraint, so a constraint is detached exactly when no reachable element
// declaration owns it. It charges one dependency step per declaration and
// type visited and per model and particle read.
func (c *compiler) detachUnreachableComponents() error {
	r := componentReach{
		elements:     make([]bool, len(c.rt.Elements)),
		complexTypes: make([]bool, len(c.rt.ComplexTypes)),
		models:       make([]bool, len(c.rt.Models)),
	}
	for i := range c.rt.ComplexTypes {
		if c.rt.ComplexTypes[i].Scope == declarationScopeGlobal {
			if err := c.reachComplexType(&r, ComplexTypeID(i)); err != nil {
				return err
			}
		}
	}
	for i := range c.rt.Elements {
		if c.rt.Elements[i].Scope == declarationScopeGlobal {
			if err := c.reachElement(&r, ElementID(i)); err != nil {
				return err
			}
		}
	}
	for next := 0; next < len(r.queue); next++ {
		if err := c.reachParticleElements(&r, r.queue[next]); err != nil {
			return err
		}
	}
	for i := range c.rt.ComplexTypes {
		c.rt.ComplexTypes[i].Detached = !r.complexTypes[i]
	}
	for i := range c.rt.Identities {
		c.rt.Identities[i].Detached = true
	}
	for i := range c.rt.Elements {
		if !r.elements[i] {
			c.rt.Elements[i].Identity = nil
		}
		for _, id := range c.rt.Elements[i].Identity {
			if !validIdentityConstraintID(id, len(c.rt.Identities)) {
				return xsderrors.InternalInvariant("element declaration references invalid identity constraint")
			}
			c.rt.Identities[id].Detached = false
		}
	}
	c.rt.ReachableElements = r.elements
	return nil
}

func (c *compiler) reachElement(r *componentReach, id ElementID) error {
	if r.elements[id] {
		return nil
	}
	r.elements[id] = true
	if err := c.dependencyWork.spend(1); err != nil {
		return err
	}
	if typ, ok := c.rt.Elements[id].Type.Complex(); ok {
		return c.reachComplexType(r, typ)
	}
	return nil
}

func (c *compiler) reachComplexType(r *componentReach, id ComplexTypeID) error {
	if !validComplexTypeID(id, len(r.complexTypes)) {
		return xsderrors.InternalInvariant("element declaration references invalid complex type")
	}
	if r.complexTypes[id] {
		return nil
	}
	r.complexTypes[id] = true
	if err := c.dependencyWork.spend(1); err != nil {
		return err
	}
	return r.reachModel(c.rt.ComplexTypes[id].Content)
}

func (r *componentReach) reachModel(id contentModelID) error {
	if !validContentModelID(id, len(r.models)) {
		return xsderrors.InternalInvariant("content model reference is invalid")
	}
	if !r.models[id] {
		r.models[id] = true
		r.queue = append(r.queue, id)
	}
	return nil
}

func (c *compiler) reachParticleElements(r *componentReach, id contentModelID) error {
	particles, err := chargedParticles(c.dependencyWork.spend, c.rt.Models[id])
	if err != nil {
		return err
	}
	for _, particle := range particles {
		switch particle.Kind { //nolint:exhaustive // a wildcard reaches only global declarations, which are roots.
		case particleElement:
			if !validElementID(particle.Element, len(r.elements)) {
				return xsderrors.InternalInvariant("particle references invalid element")
			}
			err = c.reachElement(r, particle.Element)
		case particleModel:
			err = r.reachModel(particle.Model)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// validateReachableHolders checks that what validation can read names no
// detached or unreachable component: no global type is detached, a reachable
// element declaration's type is not detached, and every element a published
// content model's table names is reachable. The tables of entered models
// were charged edge by edge when they were projected, and this reads each
// edge, row and xs:all term once, so it charges nothing.
func validateReachableHolders(build *schemaBuild, entered []int) error {
	if len(build.ReachableElements) != len(build.Elements) {
		return xsderrors.InternalInvariant("element reachability does not cover the element declarations")
	}
	for _, typ := range build.ComplexTypes {
		if typ.Detached && typ.Scope == declarationScopeGlobal {
			return xsderrors.InternalInvariant("global complex type is detached")
		}
	}
	for i, decl := range build.Elements {
		typ, ok := decl.Type.Complex()
		if ok && build.ReachableElements[i] && validComplexTypeID(typ, len(build.ComplexTypes)) && build.ComplexTypes[typ].Detached {
			return xsderrors.InternalInvariant("reachable element declaration has a detached complex type")
		}
	}
	unreachable := func(p compiledParticleRead) bool {
		return p.Kind == particleElement && (!validElementID(p.Element, len(build.Elements)) || !build.ReachableElements[p.Element])
	}
	content := &build.CompiledContent
	for _, root := range entered {
		model, run := content.Models[root], content.Runs[root]
		for _, term := range model.All {
			if unreachable(term.Particle) {
				return unreachableContentElementError()
			}
		}
		for _, edge := range run {
			if unreachable(edge.Particle) {
				return unreachableContentElementError()
			}
		}
		for _, row := range model.Rows {
			if row.Counted && unreachable(row.CountParticle) {
				return unreachableContentElementError()
			}
			if run != nil {
				continue
			}
			for _, edge := range row.Edges {
				if unreachable(edge.Particle) {
					return unreachableContentElementError()
				}
			}
		}
	}
	return nil
}

func unreachableContentElementError() error {
	return xsderrors.InternalInvariant("published content model names an unreachable element declaration")
}
