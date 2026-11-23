package schema

import (
	"iter"

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

// newPublishedElements numbers the reached declarations of build and checks
// that every content table validation can enter, entered, names only reached
// ones (validateReachableHolders).
func newPublishedElements(build *schemaBuild, entered []int) (publishedElements, error) {
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
	published := publishedElements{ids: ids, count: count}
	if err := validateReachableHolders(build, entered, published); err != nil {
		return publishedElements{}, err
	}
	return published, nil
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

// rewriteContent rewrites the element particles of the published content
// tables in place from build to published IDs (elementRewrite).
func (p publishedElements) rewriteContent(models []compiledModelRead, runs [][]compiledModelEdgeRead) {
	for i := range models {
		for particle := range publishedParticles(&models[i], runs[i]) {
			if particle.Kind == particleElement {
				particle.Element = p.ids[particle.Element]
			}
		}
	}
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
// detached or unreachable component: no global type is detached, a reachable
// element declaration's type is not detached, and every element a published
// content model's table names has a published ID. The tables of entered
// models were charged edge by edge when they were projected, and this reads
// each edge, row and xs:all term once, so it charges nothing.
func validateReachableHolders(build *schemaBuild, entered []int, published publishedElements) error {
	if err := validateGlobalComplexTypesAttached(build.ComplexTypes); err != nil {
		return err
	}
	for i := range build.Elements {
		if err := validateReachableElementType(build, ElementID(i)); err != nil {
			return err
		}
	}
	for _, root := range entered {
		if err := validateReachableContent(build, published, &build.CompiledContent.Models[root], build.CompiledContent.Runs[root]); err != nil {
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

// validateReachableContent checks the particles of one published table. run
// is the edge run every row's edges are a window of, or nil when each row
// holds its own edges; each window is checked to share the run's storage, so
// reading the run once reads every row's edges.
func validateReachableContent(build *schemaBuild, published publishedElements, model *compiledModelRead, run []compiledModelEdgeRead) error {
	if err := validateRowsInRun(model.Rows, run); err != nil {
		return err
	}
	for p := range publishedParticles(model, run) {
		switch {
		case p.Kind != particleElement:
			continue
		case !validElementID(p.Element, len(build.Elements)):
			return xsderrors.InternalInvariant("published content model references an invalid element declaration")
		}
		if _, ok := published.id(p.Element); !ok {
			return xsderrors.InternalInvariant("published content model names an unreachable element declaration")
		}
	}
	return nil
}

func validateRowsInRun(rows []compiledModelRowRead, run []compiledModelEdgeRead) error {
	if run == nil {
		return nil
	}
	for _, row := range rows {
		if err := validateRowEdgesInRun(row.Edges, run); err != nil {
			return err
		}
	}
	return nil
}

func validateRowEdgesInRun(edges, run []compiledModelEdgeRead) error {
	if len(edges) == 0 {
		return nil
	}
	start := uint64(edges[0].To) - 1
	if edges[0].To == 0 || start+uint64(len(edges)) > uint64(len(run)) || &edges[0] != &run[start] {
		return xsderrors.InternalInvariant("published content-model row edges are not a window of the model's edge run")
	}
	return nil
}

// publishedParticles yields each particle of one published table once: the
// edges of run when the rows' edges are windows of it, else each row's edges,
// then each counted row's count particle and each xs:all term.
func publishedParticles(model *compiledModelRead, run []compiledModelEdgeRead) iter.Seq[*compiledParticleRead] {
	return func(yield func(*compiledParticleRead) bool) {
		if yieldEdgeParticles(run, yield) && yieldRowParticles(model.Rows, run, yield) {
			for i := range model.All {
				if !yield(&model.All[i].Particle) {
					return
				}
			}
		}
	}
}

func yieldEdgeParticles(edges []compiledModelEdgeRead, yield func(*compiledParticleRead) bool) bool {
	for i := range edges {
		if !yield(&edges[i].Particle) {
			return false
		}
	}
	return true
}

// yieldRowParticles yields the particles of rows, with their edges only when
// they are not windows of run.
func yieldRowParticles(rows []compiledModelRowRead, run []compiledModelEdgeRead, yield func(*compiledParticleRead) bool) bool {
	for i := range rows {
		if run == nil && !yieldEdgeParticles(rows[i].Edges, yield) {
			return false
		}
		if rows[i].Counted && !yield(&rows[i].CountParticle) {
			return false
		}
	}
	return true
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
