package schema

import (
	"slices"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

// componentReach is the walk from the components an instance can name to the
// components validation can assess. An instance names only global
// declarations and types: a root element, xsi:type, a wildcard and a
// substitution group member all resolve by global name. From there validation
// reaches an element declaration's type, and a complex type's content model
// with every element particle in it and in its nested models, its text type
// and the types of its attribute uses. A simple type reaches its base, list
// item and union members, which its evaluation and diagnostics read. Each
// model, attribute-use set and simple type is visited once by ID, so a group
// or attribute group shared by many types is read once.
//
// The walk detaches every complex type before it starts and attaches each one
// it reaches, so complexTypeMetadata.Detached is its complex-type mark. The
// other marks share one allocation. Builtin simple types are always kept and
// name only builtins, so simpleTypes marks the user types, indexed from
// value.BuiltinTypeCount. A user simple type is walked depth-first: the value
// builder completed it only after its base, list item and union members, at a
// greater dependency depth, so the recursion is no deeper than
// maxComponentDependencyDepth.
type componentReach struct {
	elements      []bool
	models        []bool
	attributeSets []bool
	simpleTypes   []bool
	queue         []contentModelID
}

// detachUnreachableComponents marks the components no instance element
// reaches. It is the one writer of complexTypeMetadata.Detached,
// identityConstraintSpec.Detached, schemaBuild.ReachableElements and
// schemaBuild.ReachableSimpleTypes. A
// constraint is detached when its declarer is unreached or it has none.
// Element declarations keep their identity lists, which the restriction
// checks compare; publication omits unreached declarations
// (publishedElements).
//
// The walk charges no budget. It reads the particles of each model once, and
// only models complexTypeReachableModels already read and charged under
// MaxContentModelAnalysisSteps, and it visits each element declaration and
// complex type once, from a global declaration or from such a particle, so
// its work is bounded by steps already spent. It reads the uses of each
// attribute-use set once, uses its compilation charged under
// MaxSchemaDependencySteps, and visits each simple type once, reading its
// base, list item and union members, which its definition charged when it
// was compiled.
func (c *compiler) detachUnreachableComponents() error {
	r, err := c.reachFromGlobals()
	if err != nil {
		return err
	}
	for i := range c.rt.Identities {
		declarer := c.rt.Identities[i].Declarer
		c.rt.Identities[i].Detached = declarer == NoElement || !validElementID(declarer, len(r.elements)) || !r.elements[declarer]
	}
	c.rt.ReachableElements = r.elements
	c.rt.ReachableSimpleTypes = r.simpleTypes
	return nil
}

func (c *compiler) reachFromGlobals() (componentReach, error) {
	userSimpleTypes := len(c.rt.SimpleTypes) - int(value.BuiltinTypeCount)
	if userSimpleTypes < 0 {
		return componentReach{}, xsderrors.InternalInvariant("simple type table does not hold the builtins")
	}
	for i := range c.rt.ComplexTypes {
		c.rt.ComplexTypes[i].Detached = true
	}
	marks := make([]bool, len(c.rt.Elements)+len(c.rt.Models)+len(c.rt.AttributeUseSets)+userSimpleTypes)
	take := func(n int) []bool {
		taken := marks[:n:n]
		marks = marks[n:]
		return taken
	}
	r := componentReach{
		elements:      take(len(c.rt.Elements)),
		models:        take(len(c.rt.Models)),
		attributeSets: take(len(c.rt.AttributeUseSets)),
		simpleTypes:   take(userSimpleTypes),
	}
	if err := c.reachGlobalSimpleTypes(&r); err != nil {
		return componentReach{}, err
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

// reachGlobalSimpleTypes roots the simple types validation can name without
// an element: the global types that xsi:type names, and the types of the
// global and intrinsic xsi attribute declarations.
func (c *compiler) reachGlobalSimpleTypes(r *componentReach) error {
	for i := int(value.BuiltinTypeCount); i < len(c.rt.SimpleTypes); i++ {
		if c.rt.SimpleTypes[i].Scope != declarationScopeGlobal {
			continue
		}
		if err := c.reachSimpleType(r, SimpleTypeID(i)); err != nil {
			return err
		}
	}
	for i := range c.rt.Attributes {
		if err := c.reachSimpleType(r, c.rt.Attributes[i].Type); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) reachSimpleType(r *componentReach, id SimpleTypeID) error {
	if id == NoSimpleType || id < value.BuiltinTypeCount {
		return nil
	}
	user := int(id - value.BuiltinTypeCount)
	if user >= len(r.simpleTypes) {
		return xsderrors.InternalInvariant("simple type reference is invalid")
	}
	if r.simpleTypes[user] {
		return nil
	}
	r.simpleTypes[user] = true
	spec := &c.rt.SimpleTypes[id].ValueSpec
	if err := c.reachSimpleType(r, spec.Base); err != nil {
		return err
	}
	if err := c.reachSimpleType(r, spec.ListItem); err != nil {
		return err
	}
	for _, member := range spec.Union {
		if err := c.reachSimpleType(r, member); err != nil {
			return err
		}
	}
	return nil
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
	typ := c.rt.Elements[id].Type
	if simple, ok := typ.Simple(); ok {
		return c.reachSimpleType(r, simple)
	}
	if complexType, ok := typ.Complex(); ok {
		return c.reachComplexType(r, complexType)
	}
	return nil
}

func (c *compiler) reachComplexType(r *componentReach, id ComplexTypeID) error {
	if !validComplexTypeID(id, len(c.rt.ComplexTypes)) {
		return xsderrors.InternalInvariant("element declaration references invalid complex type")
	}
	if !c.rt.ComplexTypes[id].Detached {
		return nil
	}
	c.rt.ComplexTypes[id].Detached = false
	typ := &c.rt.ComplexTypes[id]
	if err := c.reachSimpleType(r, typ.TextType); err != nil {
		return err
	}
	if err := c.reachAttributeUseSet(r, typ.Attrs); err != nil {
		return err
	}
	return r.reachModel(typ.Content)
}

func (c *compiler) reachAttributeUseSet(r *componentReach, id attributeUseSetID) error {
	if !validAttributeUseSetID(id, len(r.attributeSets)) {
		return xsderrors.InternalInvariant("complex type references invalid attribute use set")
	}
	if r.attributeSets[id] {
		return nil
	}
	r.attributeSets[id] = true
	for _, use := range c.rt.AttributeUseSets[id].Uses {
		if err := c.reachSimpleType(r, use.Type); err != nil {
			return err
		}
	}
	return nil
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

// id returns the published ID of the declaration buildID names, and false
// when buildID names no declaration or an unreached one.
func (p publishedElements) id(buildID ElementID) (ElementID, bool) {
	if !validElementID(buildID, len(p.ids)) {
		return NoElement, false
	}
	id := p.ids[buildID]
	return id, validElementID(id, p.count)
}

// keeps reports whether the declaration buildID names is published under
// buildID itself, as every declaration below the first unreached one is.
func (p publishedElements) keeps(buildID ElementID) bool {
	return validElementID(buildID, len(p.ids)) && p.ids[buildID] == buildID
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

// publishedModel returns one entered table over published IDs, rejecting a
// particle that names an invalid or unreached declaration. run is the edge
// run every row's edges are a window of, or nil when each row holds its own
// edges. A table that names only declarations the numbering keeps is already
// over published IDs and is returned as it is, sharing its storage with the
// build. Any other table is copied: the copy shares no particle storage with
// model, so reading model leaves it unchanged however its rows share edges,
// and a run is copied once, keeping each row's window of it. No published
// table mixes build and published IDs. The tables of entered models were
// charged edge by edge when they were projected, and this reads each edge,
// row and xs:all term at most twice, so it charges nothing.
func (p publishedElements) publishedModel(build *schemaBuild, model compiledModelRead, run []compiledModelEdgeRead) (compiledModelRead, error) {
	if p.keepsModel(&model, run) {
		return model, nil
	}
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

// keepsModel reports whether every row of a run model holds a window of run
// and every element particle of model names a declaration the numbering
// keeps.
func (p publishedElements) keepsModel(model *compiledModelRead, run []compiledModelEdgeRead) bool {
	if !p.keepsEdges(run) {
		return false
	}
	for i := range model.Rows {
		row := &model.Rows[i]
		if !p.keepsRowEdges(row.Edges, run) || row.Counted && !p.keepsParticle(row.CountParticle) {
			return false
		}
	}
	for _, term := range model.All {
		if !p.keepsParticle(term.Particle) {
			return false
		}
	}
	return true
}

// keepsRowEdges reports whether a row of a run model is a window of run, or
// whether every particle of another row's edges names a kept declaration.
func (p publishedElements) keepsRowEdges(row, run []compiledModelEdgeRead) bool {
	if run != nil {
		return isRunWindow(row, run)
	}
	return p.keepsEdges(row)
}

func (p publishedElements) keepsEdges(edges []compiledModelEdgeRead) bool {
	for _, edge := range edges {
		if !p.keepsParticle(edge.Particle) {
			return false
		}
	}
	return true
}

func (p publishedElements) keepsParticle(particle compiledParticleRead) bool {
	return particle.Kind != particleElement || p.keeps(particle.Element)
}

// isRunWindow reports whether row is empty or a window of run located by its
// first edge's target.
func isRunWindow(row, run []compiledModelEdgeRead) bool {
	if len(row) == 0 {
		return true
	}
	start := uint64(row[0].To) - 1
	return row[0].To != 0 && start+uint64(len(row)) <= uint64(len(run)) && &row[0] == &run[start]
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
		if !isRunWindow(row, run) {
			return nil, nil, xsderrors.InternalInvariant("published content-model row edges are not a window of the model's edge run")
		}
		start := uint64(row[0].To) - 1
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

// validatePublishedSimpleTypes checks that every simple type a published read
// names is one build.ReachableSimpleTypes marks, so sealing the value program
// keeping only the reached types leaves no read naming a type without a
// definition. When every simple type is reached no read can name an unreached
// one, so the reads are not scanned.
func validatePublishedSimpleTypes(p *schemaProgram, build *schemaBuild) error {
	r := reachedSimpleTypes(build.ReachableSimpleTypes)
	switch {
	case len(r) != len(build.SimpleTypes)-int(value.BuiltinTypeCount):
		return xsderrors.InternalInvariant("reachable simple types do not cover the simple types")
	case !slices.Contains(r, false):
		return nil
	case !r.keepElementTypes(p.Elements.meta):
		return xsderrors.InternalInvariant("published element declaration names an unreached simple type")
	case !r.keepTextTypes(p.ComplexTypes):
		return xsderrors.InternalInvariant("published complex type names an unreached text type")
	case !r.keepAttributeUseTypes(p.AttributeUseSets):
		return xsderrors.InternalInvariant("published attribute use names an unreached simple type")
	case !r.keepAttributeDeclTypes(p.Attributes):
		return xsderrors.InternalInvariant("published attribute declaration names an unreached simple type")
	case !r.keepGlobalTypes(p.GlobalTypes):
		return xsderrors.InternalInvariant("global simple type is unreached")
	}
	return nil
}

// reachedSimpleTypes is schemaBuild.ReachableSimpleTypes.
type reachedSimpleTypes []bool

func (r reachedSimpleTypes) keeps(id SimpleTypeID) bool {
	if id == NoSimpleType || id < value.BuiltinTypeCount {
		return true
	}
	user := uint64(id - value.BuiltinTypeCount)
	return user < uint64(len(r)) && r[user]
}

func (r reachedSimpleTypes) keepsType(typ TypeID) bool {
	id, simple := typ.Simple()
	return !simple || r.keeps(id)
}

func (r reachedSimpleTypes) keepElementTypes(meta []elementReadMeta) bool {
	for i := range meta {
		if !r.keepsType(meta[i].typ) {
			return false
		}
	}
	return true
}

func (r reachedSimpleTypes) keepTextTypes(types []complexTypeRead) bool {
	for i := range types {
		if !r.keeps(types[i].textType) {
			return false
		}
	}
	return true
}

func (r reachedSimpleTypes) keepAttributeUseTypes(sets []AttributeUseSetRead) bool {
	for i := range sets {
		for _, use := range sets[i].uses {
			if !r.keeps(use.typ) {
				return false
			}
		}
	}
	return true
}

func (r reachedSimpleTypes) keepAttributeDeclTypes(decls []AttributeDeclRead) bool {
	for i := range decls {
		if !r.keeps(decls[i].typ) {
			return false
		}
	}
	return true
}

func (r reachedSimpleTypes) keepGlobalTypes(types map[QName]TypeID) bool {
	for _, typ := range types {
		if !r.keepsType(typ) {
			return false
		}
	}
	return true
}

func uncharged(int) error { return nil }
