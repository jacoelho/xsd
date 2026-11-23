package schema

import (
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

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
// identityConstraintSpec.Detached and schemaBuild.ReachableElements. A
// constraint is detached when its declarer is unreached or it has none.
// Element declarations keep their identity lists, which the restriction
// checks compare; publication omits unreached declarations
// (publishedElements).
//
// The walk charges no budget. It reads the particles of each model once, and
// only models complexTypeReachableModels already read and charged under
// MaxContentModelAnalysisSteps, and it visits each element declaration and
// complex type once, from a global declaration or from such a particle, so
// its work is bounded by steps already spent.
func (c *compiler) detachUnreachableComponents() error {
	r, err := c.reachFromGlobals()
	if err != nil {
		return err
	}
	for i := range c.rt.ComplexTypes {
		c.rt.ComplexTypes[i].Detached = !r.complexTypes[i]
	}
	for i := range c.rt.Identities {
		declarer := c.rt.Identities[i].Declarer
		c.rt.Identities[i].Detached = declarer == NoElement || !validElementID(declarer, len(r.elements)) || !r.elements[declarer]
	}
	c.rt.ReachableElements = r.elements
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

// publishedElements numbers the reached element declarations densely in
// declaration order: the published element read table holds them at those
// IDs, and every published holder names them by those IDs. The numbering is
// monotone, so an order by element ID, such as the member tie-break of the
// substitution table, survives it. An unreached declaration is not published
// and has no published ID.
type publishedElements struct {
	ids   []ElementID
	count int
}

// newPublishedElements numbers the reached declarations of build.
func newPublishedElements(build *schemaBuild) (publishedElements, error) {
	if len(build.ReachableElements) != len(build.Elements) {
		return publishedElements{}, xsderrors.InternalInvariant("element reachability does not cover the element declarations")
	}
	ids := make([]ElementID, len(build.ReachableElements))
	count := 0
	for i, reached := range build.ReachableElements {
		ids[i] = NoElement
		if reached {
			ids[i] = ElementID(count) // #nosec G115 -- count is below len(build.Elements), an element-declaration table length.
			count++
		}
	}
	return publishedElements{ids: ids, count: count}, nil
}

// id returns the published ID of the declaration build names, and false when
// build names no declaration or an unreached one.
func (p publishedElements) id(build ElementID) (ElementID, bool) {
	if !validElementID(build, len(p.ids)) {
		return NoElement, false
	}
	id := p.ids[build]
	return id, validElementID(id, p.count)
}

// publishedElementMaps returns the global element map and the substitution
// table over published IDs. Every global declaration is reached, so each has
// one unless the build is corrupt.
func publishedElementMaps(build *schemaBuild, published publishedElements) (map[QName]ElementID, substitutionTable, error) {
	globals := make(map[QName]ElementID, len(build.GlobalElements))
	for name, id := range build.GlobalElements {
		publishedID, ok := published.id(id)
		if !ok {
			return nil, substitutionTable{}, xsderrors.InternalInvariant("global element declaration has no published ID")
		}
		globals[name] = publishedID
	}
	substitutions, err := build.Substitutions.published(published)
	if err != nil {
		return nil, substitutionTable{}, err
	}
	return globals, substitutions, nil
}

// validateReachableHolders checks that what validation can read names no
// detached component: no global type is detached and a reachable element
// declaration's type is not detached. publishedContentModels checks the
// elements the content tables name.
func validateReachableHolders(build *schemaBuild) error {
	if err := validateGlobalComplexTypesAttached(build.ComplexTypes); err != nil {
		return err
	}
	for i := range build.Elements {
		if err := validateReachableElementType(build, ElementID(i)); err != nil {
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

func validateReachableElementType(build *schemaBuild, id ElementID) error {
	typ, ok := build.Elements[id].Type.Complex()
	if build.ReachableElements[id] && ok && validComplexTypeID(typ, len(build.ComplexTypes)) && build.ComplexTypes[typ].Detached {
		return xsderrors.InternalInvariant("reachable element declaration has a detached complex type")
	}
	return nil
}

// publishedModel returns a copy of one entered table whose element particles
// name published IDs, rejecting a particle that names an invalid or unreached
// declaration. run is the edge run every row's edges are a window of, or nil
// when each row holds its own edges. The copy shares no particle storage with
// model, so reading model leaves it unchanged however its rows share edges, and
// a run is copied once, keeping each row's window of it. The tables of entered
// models were charged edge by edge when they were projected, and the copy
// reads each edge, row and xs:all term once, so it charges nothing.
func (p publishedElements) publishedModel(build *schemaBuild, model compiledModelRead, run []compiledModelEdgeRead) (compiledModelRead, error) {
	size := len(run)
	if run == nil {
		size = rowEdgeCount(model.Rows)
	}
	edges, err := p.publishedEdges(build, run, size)
	if err != nil {
		return compiledModelRead{}, err
	}
	if model.Rows, err = p.publishedRows(build, model.Rows, edges, run); err != nil {
		return compiledModelRead{}, err
	}
	model.All = slices.Clone(model.All)
	for i := range model.All {
		if model.All[i].Particle, err = p.publishedParticle(build, model.All[i].Particle); err != nil {
			return compiledModelRead{}, err
		}
	}
	return model, nil
}

// publishedRows returns a copy of rows whose edges are taken from edges, the
// published run when run is set and otherwise an empty array with capacity
// for every row's edges.
func (p publishedElements) publishedRows(build *schemaBuild, rows []compiledModelRowRead, edges, run []compiledModelEdgeRead) ([]compiledModelRowRead, error) {
	rows = slices.Clone(rows)
	var err error
	for i := range rows {
		if rows[i].Edges, edges, err = p.publishedRowEdges(build, rows[i].Edges, edges, run); err != nil {
			return nil, err
		}
		if rows[i].Counted {
			if rows[i].CountParticle, err = p.publishedParticle(build, rows[i].CountParticle); err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func rowEdgeCount(rows []compiledModelRowRead) int {
	count := 0
	for i := range rows {
		count += len(rows[i].Edges)
	}
	return count
}

// publishedEdges returns source over published IDs in a new array of
// capacity size.
func (p publishedElements) publishedEdges(build *schemaBuild, source []compiledModelEdgeRead, size int) ([]compiledModelEdgeRead, error) {
	if size == 0 {
		return nil, nil
	}
	edges := make([]compiledModelEdgeRead, len(source), size)
	for i, edge := range source {
		particle, err := p.publishedParticle(build, edge.Particle)
		if err != nil {
			return nil, err
		}
		edges[i] = compiledModelEdgeRead{Particle: particle, To: edge.To}
	}
	return edges, nil
}

// publishedRowEdges returns the published edges of one row and the array the
// next row's edges are taken from. A row of a run model takes its window of
// the published run, once checked to be a window of run; any other row's
// edges are appended to edges, whose capacity holds every row's edges.
func (p publishedElements) publishedRowEdges(build *schemaBuild, row, edges, run []compiledModelEdgeRead) (_, next []compiledModelEdgeRead, err error) {
	if len(row) == 0 {
		return nil, edges, nil
	}
	if run != nil {
		start := uint64(row[0].To) - 1
		if row[0].To == 0 || start+uint64(len(row)) > uint64(len(run)) || &row[0] != &run[start] {
			return nil, nil, xsderrors.InternalInvariant("published content-model row edges are not a window of the model's edge run")
		}
		end := start + uint64(len(row))
		return edges[start:end:end], edges, nil
	}
	start := len(edges)
	for _, edge := range row {
		particle, err := p.publishedParticle(build, edge.Particle)
		if err != nil {
			return nil, nil, err
		}
		edges = append(edges, compiledModelEdgeRead{Particle: particle, To: edge.To})
	}
	return edges[start:len(edges):len(edges)], edges, nil
}

func (p publishedElements) publishedParticle(build *schemaBuild, particle compiledParticleRead) (compiledParticleRead, error) {
	if particle.Kind != particleElement {
		return particle, nil
	}
	if !validElementID(particle.Element, len(build.Elements)) {
		return compiledParticleRead{}, xsderrors.InternalInvariant("published content model references an invalid element declaration")
	}
	id, ok := p.id(particle.Element)
	if !ok {
		return compiledParticleRead{}, xsderrors.InternalInvariant("published content model names an unreachable element declaration")
	}
	particle.Element = id
	return particle, nil
}

// validatePublishedIdentityConstraints checks that the published element
// reads name exactly the constraints that are not detached, each once, so
// every constraint with an evaluation program has an element that opens its
// scope and no element names a constraint without one.
func validatePublishedIdentityConstraints(elements *elementReadTable, identities []identityConstraintSpec) error {
	named := make([]bool, len(identities))
	for _, id := range elements.identities {
		if !validIdentityConstraintID(id, len(identities)) || named[id] {
			return xsderrors.InternalInvariant("published element reads name an identity constraint invalidly or twice")
		}
		named[id] = true
	}
	for id := range identities {
		if identities[id].Detached == named[id] {
			return xsderrors.InternalInvariant("published element reads disagree with identity constraint detachment")
		}
	}
	return nil
}

func uncharged(int) error { return nil }
