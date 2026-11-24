package schema

import (
	"cmp"
	"encoding/xml"
	"slices"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) contentModel(id contentModelID, msg string) (contentModelNode, error) {
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return contentModelNode{}, xsderrors.InternalInvariant(msg, nil)
	}
	return model, nil
}

// modelUse is what a compiled model group's record is used as.
type modelUse uint8

const (
	// modelTerm is the term of a particle: the record has a unit occurrence
	// and the particle carries the occurrence.
	modelTerm modelUse = iota
	// modelRoot is the root of a {content type}: the record carries the
	// occurrence of the content type's particle.
	modelRoot
)

// compiledModelGroup is the model a model group node was compiled to, or
// noContentModel for a zero-count compositor, and the use it was compiled for.
type compiledModelGroup struct {
	id  contentModelID
	use modelUse
}

// compileModel compiles n, the model group child of a complex type definition,
// for use and returns its model and the occurrence of its particle.
func (c *compiler) compileModel(ctx *schemaSetDocument, n *schemaNode, use modelUse, owner ComplexTypeID) (contentModelID, occurrence, error) {
	source := semanticSource[*schemaGroupSource](&n.semantic)
	if source == nil || !source.Ref.Lexical.Present {
		return c.compileCompositor(ctx, n, use, owner)
	}
	id, occurs, err := c.compileModelGroupRef(ctx, n, source.Ref.Name, nil)
	if err != nil || id == noContentModel || use == modelTerm || occurs.IsExactlyOne() {
		return id, occurs, err
	}
	id, err = c.applyModelGroupOccurrence(n, id, occurs, owner)
	return id, occurs, err
}

// compileCompositor compiles the compositor n for use and returns its model
// and the occurrence of its particle. A zero-count sequence or choice has no
// model; its children are deferred. A compositor node is compiled for one use
// only, which existingModel enforces: a complex type's own model group child
// as a root or a term of its extension sequence, and every other compositor as
// a term.
func (c *compiler) compileCompositor(ctx *schemaSetDocument, n *schemaNode, use modelUse, owner ComplexTypeID) (contentModelID, occurrence, error) {
	occurs, err := particleOccurrence(n)
	if err != nil {
		return noContentModel, occurrence{}, err
	}
	if id, done, existingErr := c.existingModel(ctx, n, use); done || existingErr != nil {
		return id, occurs, existingErr
	}
	if (n.local == vocab.XSDElemSequence || n.local == vocab.XSDElemChoice) && occurs.IsAbsent() {
		if err = c.spendComponentDependency(n); err != nil {
			return noContentModel, occurrence{}, err
		}
		// Cache only the parent's absence. Its descendants retain their own
		// obligations after active group expansion unwinds.
		c.modelDone[nodeKey(ctx, n)] = compiledModelGroup{id: noContentModel, use: use}
		c.deferredModelChildren = append(c.deferredModelChildren, schemaComponent{ctx: ctx, id: n.id})
		return noContentModel, occurs, nil
	}
	record := contentModelNode{Occurs: occurrence{Min: 1, Max: 1}}
	if use == modelRoot {
		record.Occurs = occurs
	}
	id, err := c.compileModelComponent(ctx, n, record, use, owner)
	return id, occurs, err
}

func (c *compiler) compileModelComponent(ctx *schemaSetDocument, n *schemaNode, record contentModelNode, use modelUse, owner ComplexTypeID) (result contentModelID, err error) {
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return noContentModel, dependencyErr
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return noContentModel, err
	}
	defer scope.leave(&err)
	id, err := c.addModel(contentModelNode{}, n, owner)
	if err != nil {
		return noContentModel, err
	}
	c.modelDone[nodeKey(ctx, n)] = compiledModelGroup{id: id, use: use}
	c.compilingModel[nodeKey(ctx, n)] = true
	defer delete(c.compilingModel, nodeKey(ctx, n))
	m, err := c.compileModelValue(ctx, n, record, owner)
	if err != nil {
		return noContentModel, err
	}
	c.completeModel(id, m)
	return id, nil
}

func (c *compiler) existingModel(ctx *schemaSetDocument, n *schemaNode, use modelUse) (contentModelID, bool, error) {
	done, ok := c.modelDone[nodeKey(ctx, n)]
	if !ok {
		return noContentModel, false, nil
	}
	if done.use != use {
		return noContentModel, true, withSchemaCompileLocation(n, xsderrors.InternalInvariant("model group compiled for another use", nil))
	}
	if !c.compilingModel[nodeKey(ctx, n)] {
		return done.id, true, nil
	}
	err := schemaComponentRecursionError(schemaComponentModelGroup, "")
	return noContentModel, true, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileModelValue(ctx *schemaSetDocument, n *schemaNode, m contentModelNode, owner ComplexTypeID) (contentModelNode, error) {
	kind, err := modelKindForNode(n)
	if err != nil {
		return contentModelNode{}, err
	}
	m.Kind = kind
	if err := c.compileModelChildren(ctx, n, &m, owner); err != nil {
		return contentModelNode{}, err
	}
	return m, nil
}

// compileModelGroupRef compiles the model group reference n and returns the
// referenced group's own model with the reference's occurrence, or
// noContentModel for a zero-occurrence reference. When parent is not nil, n is
// a child of a model group of that kind, and the referenced group's kind is
// admitted there before the group compiles, so a misplaced reference is
// reported at its use rather than a defect inside the group.
func (c *compiler) compileModelGroupRef(ctx *schemaSetDocument, n *schemaNode, ref xml.Name, parent *modelKind) (contentModelID, occurrence, error) {
	occurs, err := particleOccurrence(n)
	if err != nil {
		return noContentModel, occurrence{}, err
	}
	// A zero-occurrence reference has no particle and therefore no term to resolve.
	if occurs.IsAbsent() {
		return noContentModel, occurs, nil
	}
	source, err := c.resolveModelGroupRef(ctx, n, ref)
	if err != nil {
		return noContentModel, occurrence{}, err
	}
	if err = admitModelGroupRef(n, source.kind, parent); err != nil {
		return noContentModel, occurrence{}, err
	}
	if c.compilingModel[nodeKey(source.raw.ctx, source.modelNode)] {
		err = schemaComponentRecursionError(schemaComponentModelGroup, c.rt.formatName(source.q))
		return noContentModel, occurrence{}, withSchemaCompileLocation(source.modelNode, err)
	}
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return noContentModel, occurrence{}, dependencyErr
	}
	id, err := c.compileGroupModel(source.raw.ctx, source.modelNode)
	if err != nil {
		return noContentModel, occurrence{}, err
	}
	if source.kind == modelAll {
		if err = validateAllModelOccurrence(occurs); err != nil {
			return noContentModel, occurrence{}, withSchemaCompileLocation(n, err)
		}
	}
	return id, occurs, nil
}

// compileGroupModel compiles modelNode, the model group of a named group, as
// the term that every reference to the group names. Its models belong to no
// complex type.
func (c *compiler) compileGroupModel(ctx *schemaSetDocument, modelNode *schemaNode) (contentModelID, error) {
	id, _, err := c.compileCompositor(ctx, modelNode, modelTerm, noComplexType)
	return id, err
}

// admitModelGroupRef admits a group of kind, referenced by n, as a child of a
// model group of kind parent; a reference with no parent is admitted.
func admitModelGroupRef(n *schemaNode, kind modelKind, parent *modelKind) error {
	if parent == nil {
		return nil
	}
	return validateModelGroupChildAtNode(n, *parent, modelChildAdmissionForModelKind(kind))
}

type modelGroupRefSource struct {
	modelNode *schemaNode
	raw       schemaComponent
	q         QName
	kind      modelKind
}

func (c *compiler) resolveModelGroupRef(ctx *schemaSetDocument, n *schemaNode, ref xml.Name) (modelGroupRefSource, error) {
	q, err := c.referenceQName(ctx, n, ref)
	if err != nil {
		return modelGroupRefSource{}, err
	}
	label := c.rt.formatName(q)
	raw, ok := c.groupComponents[q]
	if !ok {
		return modelGroupRefSource{}, withSchemaCompileLocation(n, schemaComponentMissingError(schemaComponentModelGroup, label))
	}
	modelNode, err := topLevelGroupModel(raw.sourceNode())
	if err != nil {
		return modelGroupRefSource{}, err
	}
	kind, err := modelKindForNode(modelNode)
	if err != nil {
		return modelGroupRefSource{}, err
	}
	return modelGroupRefSource{raw: raw, q: q, modelNode: modelNode, kind: kind}, nil
}

// applyModelGroupOccurrence returns the root of a {content type} whose
// particle is n, a reference to the group id with a non-unit occurrence. A
// root carries its occurrence on its record, and the group's record is shared
// by every reference, so the root shares the group's particles in a record of
// its own, located at n. A violation is located at the first model, in
// source-ID order, whose check reaches it, so a violation inside the group is
// located at n only when no model with a lower ID, such as the group's own
// model reached through a unit reference, reaches the group first. The root
// belongs to owner.
func (c *compiler) applyModelGroupOccurrence(n *schemaNode, id contentModelID, occurs occurrence, owner ComplexTypeID) (contentModelID, error) {
	model, err := c.contentModel(id, "model group reference resolved missing content model")
	if err != nil {
		return noContentModel, err
	}
	model.Occurs = occurs
	return c.addModel(model, n, owner)
}

func modelKindForNode(n *schemaNode) (modelKind, error) {
	if kind, ok := schemaModelKind(n); ok {
		return kind, nil
	}
	return 0, withSchemaCompileLocation(n, xsderrors.InternalInvariant("model node has no typed model kind", nil))
}

func (c *compiler) compileModelChildren(ctx *schemaSetDocument, n *schemaNode, m *contentModelNode, owner ComplexTypeID) error {
	for child := range n.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		if err := c.appendModelChild(ctx, m, child, owner); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) appendModelChild(ctx *schemaSetDocument, m *contentModelNode, child *schemaNode, owner ComplexTypeID) error {
	switch child.local {
	case vocab.XSDElemElement:
		p, ok, err := c.compileElementParticle(ctx, child)
		if err != nil || !ok {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, c.rt.Models, m, p))
	case vocab.XSDElemAny:
		p, err := c.compileWildcardParticle(ctx, child)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, c.rt.Models, m, p))
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		return c.appendNestedModelChild(ctx, m, child, owner)
	}
	return withSchemaCompileLocation(child, xsderrors.InternalInvariant("model child has no supported typed schema kind", nil))
}

// compileNestedModelChild compiles child, a model group or group reference
// inside a model group of kind parent, after admitting its kind there. It
// returns the model the child's particle names, whose record has a unit
// occurrence, and the particle's occurrence, so a group is one record however
// its references repeat it. A compositor child's models belong to owner.
func (c *compiler) compileNestedModelChild(ctx *schemaSetDocument, child *schemaNode, parent modelKind, owner ComplexTypeID) (contentModelID, occurrence, error) {
	if source := semanticSource[*schemaGroupSource](&child.semantic); source != nil && source.Ref.Lexical.Present {
		return c.compileModelGroupRef(ctx, child, source.Ref.Name, &parent)
	}
	kind, err := modelKindForNode(child)
	if err != nil {
		return noContentModel, occurrence{}, err
	}
	if err = validateModelGroupChildAtNode(child, parent, modelChildAdmissionForModelKind(kind)); err != nil {
		return noContentModel, occurrence{}, err
	}
	return c.compileCompositor(ctx, child, modelTerm, owner)
}

func (c *compiler) appendNestedModelChild(ctx *schemaSetDocument, m *contentModelNode, child *schemaNode, owner ComplexTypeID) error {
	id, occurs, err := c.compileNestedModelChild(ctx, child, m.Kind, owner)
	if err != nil || id == noContentModel {
		return err
	}
	return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, c.rt.Models, m, modelParticle(id, occurs)))
}

// compileDeferredModelChildren gives descendants of a zero-count sequence or
// choice their ordinary component validation once named-group expansion has
// unwound. It builds no parent model, so absent siblings share no EDC or UPA
// constraint and no recursion edge is invented, and the models it creates
// belong to no complex type.
func (c *compiler) compileDeferredModelChildren(source schemaComponent) error {
	n := source.sourceNode()
	kind, err := modelKindForNode(n)
	if err != nil {
		return err
	}
	for child := range n.xsdChildren() {
		switch child.local {
		case vocab.XSDElemAnnotation:
			continue
		case vocab.XSDElemElement:
			_, _, err = c.compileElementParticle(source.ctx, child)
		case vocab.XSDElemAny:
			_, err = c.compileWildcardParticle(source.ctx, child)
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
			_, _, err = c.compileNestedModelChild(source.ctx, child, kind, noComplexType)
		default:
			return withSchemaCompileLocation(child, xsderrors.InternalInvariant("model group child order admitted xs:"+diagnostictext.Quote(child.local), nil))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func validateModelGroupChildAtNode(n *schemaNode, parent modelKind, child modelChildAdmission) error {
	return withSchemaCompileLocation(n, validateModelGroupChildAdmission(parent, child))
}

// particleOccurrence returns the occurrence the start tag of the particle n
// admitted.
func particleOccurrence(n *schemaNode) (occurrence, error) {
	source := n.semantic.particle()
	if source == nil {
		return occurrence{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("particle node has no typed particle source", nil))
	}
	return source.Occurs, nil
}

// enterContentModel opens the scope of the component that owns model id. The
// builtin anyType model has no source; its checks are set-wide.
func (c *compiler) enterContentModel(id contentModelID) (componentScope, error) {
	return c.enterComponent(c.modelSources[id])
}

// compileContentModels compiles every model reached from a complex type, in
// source-ID order and in the scope of its owner. A nested model is compiled on
// its own as well as in its ancestors' rows, so a violation or limit inside a
// group stops compilation at the first model, in source order, holding it.
// Only a root model validation can enter (entersContent) can be published, so
// only such a root keeps its published form, built over the published element
// numbering; any other root keeps the kind and the scalars of its form, and
// any other model its kind alone. A root validation cannot enter may name
// unpublished declarations, so it is projected and indexed over build IDs,
// spending what an entered root would, before it is reduced.
func (c *compiler) compileContentModels(reached []reachedModel) error {
	elements, err := newPublishedElements(c.rt.ReachableElements, len(c.rt.Elements))
	if err != nil {
		return err
	}
	roots, err := contentRootEntries(c.rt.ComplexTypes, reached)
	if err != nil {
		return err
	}
	cc := newContentModelCompiler(&c.rt, c.limits.MaxContentModelStates, &c.contentWork, &c.dependencyWork, c.contentAnalysis)
	projection := rootProjection{
		entered: newDFARowIndexAnalysis(&c.rt, &elements, c.contentWork.spend, &c.dependencyWork),
	}
	projection.unentered = projection.entered
	if !elements.identity() {
		identity := identityElements(len(c.rt.Elements))
		projection.unentered = projection.entered.over(&identity)
	}
	sources := make([]contentModelID, 0, len(reached))
	models := make([]compiledModelRead, 0, len(reached))
	runs := make([][]compiledModelEdgeRead, 0, len(reached))
	for i, r := range reached {
		model, run, err := c.compileContentModel(&cc, projection, roots[i], r)
		if err != nil {
			return err
		}
		sources = append(sources, r.id)
		models = append(models, model)
		runs = append(runs, run)
	}
	c.rt.CompiledContent = compiledContent{Sources: sources, Models: models, Runs: runs}
	if !elements.identity() {
		c.rt.CompiledContent.Elements = new(elements)
	}
	return nil
}

// rootEntry classifies a content model as the root of no complex type, the
// root only of types validation cannot enter, or a root validation enters
// (entersContent).
type rootEntry uint8

const (
	notRoot rootEntry = iota
	unenteredRoot
	enteredRoot
)

// contentRootEntries returns the rootEntry of each reached model, parallel to
// reached. Every complex type's root is reached.
func contentRootEntries(types []complexTypeMetadata, reached []reachedModel) ([]rootEntry, error) {
	entries := make([]rootEntry, len(reached))
	for i := range types {
		position, ok := slices.BinarySearchFunc(reached, types[i].Content, func(r reachedModel, id contentModelID) int {
			return cmp.Compare(r.id, id)
		})
		if !ok {
			return nil, xsderrors.InternalInvariant("complex-type content model is not reached", nil)
		}
		entry := unenteredRoot
		if entersContent(&types[i]) {
			entry = enteredRoot
		}
		entries[position] = max(entries[position], entry)
	}
	return entries, nil
}

// rootProjection projects and indexes the root models of complex types: an
// entered root over published element IDs, and any other root over build IDs.
type rootProjection struct {
	entered   *dfaRowIndexAnalysis
	unentered *dfaRowIndexAnalysis
}

// project returns the published form of an entered root, and the kind and the
// scalars of the form of any other root.
func (p rootProjection) project(entry rootEntry, compiled compiledModel, work contentModelWork) (compiledModelRead, []compiledModelEdgeRead, error) {
	index := p.entered
	if entry != enteredRoot {
		index = p.unentered
	}
	model, run, err := newCompiledModelRead(compiled, work, *index.elements)
	if err != nil {
		return compiledModelRead{}, nil, err
	}
	if err := index.indexModel(&model, run); err != nil {
		return compiledModelRead{}, nil, err
	}
	if entry != enteredRoot {
		return model.withoutTables(), nil, nil
	}
	return model, run, nil
}

func (c *compiler) compileContentModel(cc *contentModelCompiler, projection rootProjection, entry rootEntry, reached reachedModel) (_ compiledModelRead, _ []compiledModelEdgeRead, err error) {
	id := reached.id
	scope, err := c.enterContentModel(id)
	if err != nil {
		return compiledModelRead{}, nil, err
	}
	defer scope.leave(&err)
	typed := cc.forType(c.complexTypeLabel(reached.complexType))
	compiled, err := typed.compileRootContentModel(id)
	if err != nil {
		return compiledModelRead{}, nil, err
	}
	if entry == notRoot {
		return compiledModelRead{Kind: compiled.Kind}, nil, nil
	}
	return projection.project(entry, compiled, c.contentWork.spend)
}

func (c *compiler) checkCompiledModelsUPA(reached []reachedModel) error {
	upa := newContentModelUPACheck(&c.rt, len(c.rt.Models), &c.contentWork, &c.dependencyWork, c.contentAnalysis)
	for _, r := range reached {
		if err := c.checkModelUPA(upa, r); err != nil {
			return err
		}
	}
	return nil
}

// reachedModel is a content model a complex type reaches and the complex type
// a Unique Particle Attribution violation in it is reported against.
type reachedModel struct {
	id          contentModelID
	complexType ComplexTypeID
}

// complexTypeReachableModels returns the content models that Unique Particle
// Attribution and automaton construction apply to, as
// docs/spec/INTERPRETATIONS.md scopes them, in ascending ID order. The walk
// reads each model's particles through chargedParticles, in the scope of the
// model holding them. It visits the complex types' roots in complex-type order and then nested
// models breadth first, so a limit failure names the first model in that
// order; only the builtin anyType model's charge has no source and is located
// set-wide. Each reached model records the complex type a violation in it is
// reported against: the type whose definition created it (modelDefiners), so
// the message and the location name the same definition; a model a named
// group created takes the type recorded for the model whose particle first
// reaches it, and a root no definition created takes its own type.
func (c *compiler) complexTypeReachableModels() ([]reachedModel, error) {
	walk := modelReach{
		reached:      make([]bool, len(c.rt.Models)),
		complexTypes: make([]ComplexTypeID, len(c.rt.Models)),
		definers:     c.modelDefiners,
		queue:        make([]contentModelID, 0, len(c.rt.ComplexTypes)),
	}
	for i := range c.rt.ComplexTypes {
		if err := walk.reach(c.rt.ComplexTypes[i].Content, ComplexTypeID(i)); err != nil {
			return nil, err
		}
	}
	// The queue grows while it is read, so its length is re-read each step.
	for next := 0; next < len(walk.queue); {
		id := walk.queue[next]
		next++
		if err := c.reachModelParticles(&walk, id); err != nil {
			return nil, err
		}
	}
	return walk.ids(), nil
}

func (c *compiler) reachModelParticles(walk *modelReach, id contentModelID) (err error) {
	scope, err := c.enterContentModel(id)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	particles, err := chargedParticles(c.contentWork.spend, c.rt.Models[id])
	if err != nil {
		return err
	}
	return walk.reachParticles(particles, walk.complexTypes[id])
}

// modelReach is a breadth-first walk over the content models. queue holds
// each reached model once, in visit order, and complexTypes the complex type
// each reached model is reported against. definers is the compiler's
// modelDefiners, read only.
type modelReach struct {
	reached      []bool
	complexTypes []ComplexTypeID
	definers     []ComplexTypeID
	queue        []contentModelID
}

// ids returns the reached models in ascending ID order.
func (r *modelReach) ids() []reachedModel {
	ids := make([]reachedModel, 0, len(r.queue))
	for id, ok := range r.reached {
		if ok {
			ids = append(ids, reachedModel{id: contentModelID(id), complexType: r.complexTypes[id]})
		}
	}
	return ids
}

// reach records id, reported against its definer or, for a model no type
// definition created, against reachedFrom.
func (r *modelReach) reach(id contentModelID, reachedFrom ComplexTypeID) error {
	if !validContentModelID(id, len(r.reached)) {
		return xsderrors.InternalInvariant("content model reachability references missing content model", nil)
	}
	if !r.reached[id] {
		r.reached[id] = true
		r.complexTypes[id] = reachedFrom
		if definer := r.definers[id]; definer != noComplexType {
			r.complexTypes[id] = definer
		}
		r.queue = append(r.queue, id)
	}
	return nil
}

func (r *modelReach) reachParticles(particles []contentParticle, complexType ComplexTypeID) error {
	for _, particle := range particles {
		if particle.Kind != particleModel {
			continue
		}
		if err := r.reach(particle.Model, complexType); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) checkModelUPA(upa *contentModelUPACheck, reached reachedModel) (err error) {
	scope, err := c.enterContentModel(reached.id)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	return upa.checkModel(reached.id, c.complexTypeLabel(reached.complexType))
}

func (c *compiler) checkCompiledElementDeclarationsConsistent() error {
	checker := newElementDeclarationConsistencyChecker(&c.rt, c.contentWork.spend)
	for id := range c.rt.Models {
		if err := c.checkModelElementDeclarationsConsistent(checker, contentModelID(id)); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) checkModelElementDeclarationsConsistent(checker *elementDeclarationConsistencyChecker, id contentModelID) (err error) {
	scope, err := c.enterContentModel(id)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	return checker.checkModel(id)
}
