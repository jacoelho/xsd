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
// declaration owns it.
//
// The walk charges no budget, like the publication audit's re-derivation: it
// visits each element declaration, complex type, model and particle at most
// once, and compilation charged at least one step to create each, so its work
// is bounded by steps already spent.
func (c *compiler) detachUnreachableComponents() error {
	r, err := c.reachFromGlobals()
	if err != nil {
		return err
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
		if err := c.attachIdentities(c.rt.Elements[i].Identity); err != nil {
			return err
		}
	}
	c.rt.ReachableElements = r.elements
	return nil
}

func (c *compiler) attachIdentities(ids []IdentityConstraintID) error {
	for _, id := range ids {
		if !validIdentityConstraintID(id, len(c.rt.Identities)) {
			return xsderrors.InternalInvariant("element declaration references invalid identity constraint")
		}
		c.rt.Identities[id].Detached = false
	}
	return nil
}

func (c *compiler) reachFromGlobals() (componentReach, error) {
	r := componentReach{
		elements:     make([]bool, len(c.rt.Elements)),
		complexTypes: make([]bool, len(c.rt.ComplexTypes)),
		models:       make([]bool, len(c.rt.Models)),
	}
	if err := c.reachGlobalDeclarations(&r); err != nil {
		return componentReach{}, err
	}
	// The queue grows while it is read, so its length is re-read each step.
	for next := 0; next < len(r.queue); {
		id := r.queue[next]
		next++
		if err := c.reachParticleElements(&r, id); err != nil {
			return componentReach{}, err
		}
	}
	return r, nil
}

func (c *compiler) reachGlobalDeclarations(r *componentReach) error {
	for i := range c.rt.ComplexTypes {
		if c.rt.ComplexTypes[i].Scope != declarationScopeGlobal {
			continue
		}
		if err := c.reachComplexType(r, ComplexTypeID(i)); err != nil {
			return err
		}
	}
	for i := range c.rt.Elements {
		if c.rt.Elements[i].Scope != declarationScopeGlobal {
			continue
		}
		if err := c.reachElement(r, ElementID(i)); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) reachElement(r *componentReach, id ElementID) error {
	if r.elements[id] {
		return nil
	}
	r.elements[id] = true
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
	particles, err := chargedParticles(uncharged, c.rt.Models[id])
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
// detached or unreachable component: no global type is detached, an
// unreachable element declaration owns no identity constraint, a reachable
// element declaration's type is not detached, and every element a published
// content model's table names is reachable. The tables of entered models
// were charged edge by edge when they were projected, and this reads each
// edge, row and xs:all term once, so it charges nothing.
func validateReachableHolders(build *schemaBuild, entered []int) error {
	if len(build.ReachableElements) != len(build.Elements) {
		return xsderrors.InternalInvariant("element reachability does not cover the element declarations")
	}
	if err := validateGlobalComplexTypesAttached(build.ComplexTypes); err != nil {
		return err
	}
	for i := range build.Elements {
		if err := validateElementReach(build, ElementID(i)); err != nil {
			return err
		}
	}
	for _, root := range entered {
		if err := validateReachableContent(build, build.CompiledContent.Models[root], build.CompiledContent.Runs[root]); err != nil {
			return err
		}
	}
	return nil
}

func validateGlobalComplexTypesAttached(types []complexTypeMetadata) error {
	for _, typ := range types {
		if typ.Detached && typ.Scope == declarationScopeGlobal {
			return xsderrors.InternalInvariant("global complex type is detached")
		}
	}
	return nil
}

// validateElementReach checks element declaration id against whether the
// walk reached it: an unreached declaration owns no identity constraint, and
// a reached one's type is not detached.
func validateElementReach(build *schemaBuild, id ElementID) error {
	decl, reached := &build.Elements[id], build.ReachableElements[id]
	if !reached && len(decl.Identity) != 0 {
		return xsderrors.InternalInvariant("unreachable element declaration owns an identity constraint")
	}
	typ, ok := decl.Type.Complex()
	if reached && ok && validComplexTypeID(typ, len(build.ComplexTypes)) && build.ComplexTypes[typ].Detached {
		return xsderrors.InternalInvariant("reachable element declaration has a detached complex type")
	}
	return nil
}

// validateReachableContent checks the particles of one published table; run
// is the edge run its rows are windows of, or nil when each row holds its own
// edges.
func validateReachableContent(build *schemaBuild, model compiledModelRead, run []compiledModelEdgeRead) error {
	if !reachableTerms(build, model.All) || !reachableEdges(build, run) {
		return unreachableContentElementError()
	}
	for _, row := range model.Rows {
		if row.Counted && !reachableParticle(build, row.CountParticle) {
			return unreachableContentElementError()
		}
		if run == nil && !reachableEdges(build, row.Edges) {
			return unreachableContentElementError()
		}
	}
	return nil
}

func reachableTerms(build *schemaBuild, terms []compiledAllTermRead) bool {
	for _, term := range terms {
		if !reachableParticle(build, term.Particle) {
			return false
		}
	}
	return true
}

func reachableEdges(build *schemaBuild, edges []compiledModelEdgeRead) bool {
	for _, edge := range edges {
		if !reachableParticle(build, edge.Particle) {
			return false
		}
	}
	return true
}

func reachableParticle(build *schemaBuild, p compiledParticleRead) bool {
	return p.Kind != particleElement || (validElementID(p.Element, len(build.Elements)) && build.ReachableElements[p.Element])
}

func unreachableContentElementError() error {
	return xsderrors.InternalInvariant("published content model names an unreachable element declaration")
}

func uncharged(int) error { return nil }
