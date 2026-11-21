package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) contentModel(id contentModelID, msg string) (contentModelNode, error) {
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return contentModelNode{}, xsderrors.InternalInvariant(msg)
	}
	return model, nil
}

func (c *compiler) compileModel(ctx *schemaSetDocument, n *schemaNode) (contentModelID, error) {
	if source := semanticSource[*schemaGroupSource](&n.semantic); source != nil && source.Ref.Lexical.Present {
		return c.compileModelGroupRef(ctx, n, source.Ref.Name, nil)
	}
	if id, done, err := c.existingModel(ctx, n); done || err != nil {
		return id, err
	}
	occurs, err := particleOccurrence(n)
	if err != nil {
		return noContentModel, err
	}
	if (n.local == vocab.XSDElemSequence || n.local == vocab.XSDElemChoice) && occurs.IsAbsent() {
		if err := c.spendComponentDependency(n); err != nil {
			return noContentModel, err
		}
		// Cache only the parent's absence. Its descendants retain their own
		// obligations after active group expansion unwinds.
		c.modelDone[nodeKey(ctx, n)] = noContentModel
		c.deferredModelChildren = append(c.deferredModelChildren, schemaComponent{ctx: ctx, id: n.id})
		return noContentModel, nil
	}
	return c.compileModelComponent(ctx, n, occurs)
}

func (c *compiler) compileModelComponent(ctx *schemaSetDocument, n *schemaNode, occurs occurrence) (result contentModelID, err error) {
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return noContentModel, dependencyErr
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return noContentModel, err
	}
	defer scope.leave(&err)
	id, err := c.addModel(contentModelNode{}, n)
	if err != nil {
		return noContentModel, err
	}
	c.modelDone[nodeKey(ctx, n)] = id
	c.compilingModel[nodeKey(ctx, n)] = true
	defer delete(c.compilingModel, nodeKey(ctx, n))
	m, err := c.compileModelValue(ctx, n, occurs)
	if err != nil {
		return noContentModel, err
	}
	c.completeModel(id, m)
	return id, nil
}

func (c *compiler) existingModel(ctx *schemaSetDocument, n *schemaNode) (contentModelID, bool, error) {
	id, ok := c.modelDone[nodeKey(ctx, n)]
	if !ok {
		return noContentModel, false, nil
	}
	if !c.compilingModel[nodeKey(ctx, n)] {
		return id, true, nil
	}
	err := schemaComponentRecursionError(schemaComponentModelGroup, "")
	return noContentModel, true, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileModelValue(ctx *schemaSetDocument, n *schemaNode, occurs occurrence) (contentModelNode, error) {
	kind, err := modelKindForNode(n)
	if err != nil {
		return contentModelNode{}, err
	}
	m := contentModelNode{Kind: kind, Occurs: occurs}
	if err := c.compileModelChildren(ctx, n, &m); err != nil {
		return contentModelNode{}, err
	}
	return m, nil
}

// compileModelGroupRef compiles the model group reference n. When parent is
// not nil, n is a child of a model group of that kind, and the referenced
// group's kind is admitted there before the group compiles, so a misplaced
// reference is reported at its use rather than a defect inside the group.
func (c *compiler) compileModelGroupRef(ctx *schemaSetDocument, n *schemaNode, ref xml.Name, parent *modelKind) (contentModelID, error) {
	occurs, err := particleOccurrence(n)
	if err != nil {
		return noContentModel, err
	}
	// A zero-occurrence reference has no particle and therefore no term to resolve.
	if occurs.IsAbsent() {
		return noContentModel, nil
	}
	source, err := c.resolveModelGroupRef(ctx, n, ref)
	if err != nil {
		return noContentModel, err
	}
	if parent != nil {
		if err = admitModelGroupRef(n, source, *parent); err != nil {
			return noContentModel, err
		}
	}
	if c.compilingModel[nodeKey(source.raw.ctx, source.modelNode)] {
		err = schemaComponentRecursionError(schemaComponentModelGroup, c.rt.formatName(source.q))
		return noContentModel, withSchemaCompileLocation(source.modelNode, err)
	}
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return noContentModel, dependencyErr
	}
	id, err := c.compileModel(source.raw.ctx, source.modelNode)
	if err != nil {
		return noContentModel, err
	}
	return c.applyModelGroupOccurrence(n, id, occurs)
}

// admitModelGroupRef admits the model group that source resolves the
// reference n to as a child of a model group of kind parent.
func admitModelGroupRef(n *schemaNode, source modelGroupRefSource, parent modelKind) error {
	kind, err := modelKindForNode(source.modelNode)
	if err != nil {
		return err
	}
	return validateModelGroupChildAtNode(n, parent, modelChildAdmissionForModelKind(kind))
}

type modelGroupRefSource struct {
	modelNode *schemaNode
	raw       schemaComponent
	q         QName
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
	return modelGroupRefSource{raw: raw, q: q, modelNode: modelNode}, nil
}

func (c *compiler) applyModelGroupOccurrence(n *schemaNode, id contentModelID, occurs occurrence) (contentModelID, error) {
	if occurs.IsExactlyOne() {
		return id, nil
	}
	model, err := c.contentModel(id, "model group reference resolved missing content model")
	if err != nil {
		return noContentModel, err
	}
	if model.Kind == modelAll {
		if err := validateAllModelOccurrence(occurs); err != nil {
			return noContentModel, withSchemaCompileLocation(n, err)
		}
	}
	model.Occurs = occurs
	return c.addModel(model, n)
}

func modelKindForNode(n *schemaNode) (modelKind, error) {
	if kind, ok := schemaModelKind(n); ok {
		return kind, nil
	}
	return 0, withSchemaCompileLocation(n, xsderrors.InternalInvariant("model node has no typed model kind"))
}

func (c *compiler) compileModelChildren(ctx *schemaSetDocument, n *schemaNode, m *contentModelNode) error {
	for child := range n.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		if err := c.appendModelChild(ctx, m, child); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) appendModelChild(ctx *schemaSetDocument, m *contentModelNode, child *schemaNode) error {
	switch child.local {
	case vocab.XSDElemElement:
		p, ok, err := c.compileElementParticle(ctx, child)
		if err != nil || !ok {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, m, p))
	case vocab.XSDElemAny:
		p, err := c.compileWildcardParticle(ctx, child)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, m, p))
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		return c.appendNestedModelChild(ctx, m, child)
	}
	return withSchemaCompileLocation(child, xsderrors.InternalInvariant("model child has no supported typed schema kind"))
}

// compileNestedModelChild compiles child, a model group or group reference
// inside a model group of kind parent, after admitting its kind there.
func (c *compiler) compileNestedModelChild(ctx *schemaSetDocument, child *schemaNode, parent modelKind) (contentModelID, error) {
	if source := semanticSource[*schemaGroupSource](&child.semantic); source != nil && source.Ref.Lexical.Present {
		return c.compileModelGroupRef(ctx, child, source.Ref.Name, &parent)
	}
	kind, err := modelKindForNode(child)
	if err != nil {
		return noContentModel, err
	}
	if err := validateModelGroupChildAtNode(child, parent, modelChildAdmissionForModelKind(kind)); err != nil {
		return noContentModel, err
	}
	return c.compileModel(ctx, child)
}

func (c *compiler) appendNestedModelChild(ctx *schemaSetDocument, m *contentModelNode, child *schemaNode) error {
	childModelID, err := c.compileNestedModelChild(ctx, child, m.Kind)
	if err != nil || childModelID == noContentModel {
		return err
	}
	p, ok, err := c.modelParticle(childModelID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, m, p))
}

// compileDeferredModelChildren gives descendants of a zero-count sequence or
// choice their ordinary component validation once named-group expansion has
// unwound. It builds no parent model, so absent siblings share no EDC or UPA
// constraint and no recursion edge is invented.
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
			_, err = c.compileNestedModelChild(source.ctx, child, kind)
		default:
			return withSchemaCompileLocation(child, xsderrors.InternalInvariant("model group child order admitted xs:"+diagnostictext.Quote(child.local)))
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

func (c *compiler) modelParticle(id contentModelID) (contentParticle, bool, error) {
	model, err := c.contentModel(id, "model particle references missing content model")
	if err != nil {
		return contentParticle{}, false, err
	}
	occurs := model.Occurs
	if occurs.IsAbsent() {
		return contentParticle{}, false, nil
	}
	modelID := id
	if !occurs.IsExactlyOne() {
		normalized := model
		normalized.Occurs = occurrence{Min: 1, Max: 1}
		var err error
		modelID, err = c.addModel(normalized, c.modelSources[id])
		if err != nil {
			return contentParticle{}, false, err
		}
	}
	return modelParticle(modelID, occurs), true, nil
}

// particleOccurrence returns the occurrence the start tag of the particle n
// admitted.
func particleOccurrence(n *schemaNode) (occurrence, error) {
	source := n.semantic.particle()
	if source == nil {
		return occurrence{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("particle node has no typed particle source"))
	}
	return source.Occurs, nil
}

// enterContentModel opens the scope of the component that owns model id. The
// builtin anyType model has no source; its checks are set-wide.
func (c *compiler) enterContentModel(id contentModelID) (componentScope, error) {
	return c.enterComponent(c.modelSources[id])
}

// compileContentModels compiles every model reached from a complex type, in
// source-ID order and in the scope of its owner, then indexes the table. A
// nested model is compiled on its own as well as in its ancestors' rows, so a
// violation or limit inside a group stops compilation at the first model, in
// source order, holding it. Indexing is set-wide: it only spends budget, and
// locateSetWideLimit locates its limit failure.
func (c *compiler) compileContentModels(reached []bool) error {
	cc := newContentModelCompiler(&c.rt.Names, &c.rt, c.limits.MaxContentModelStates, &c.contentWork, c.contentAnalysis)
	count := 0
	for _, ok := range reached {
		if ok {
			count++
		}
	}
	sources := make([]contentModelID, 0, count)
	models := make([]compiledModel, 0, count)
	for id, ok := range reached {
		if !ok {
			continue
		}
		var compiled compiledModel
		if err := c.compileContentModel(&cc, &compiled, contentModelID(id)); err != nil {
			return err
		}
		sources = append(sources, contentModelID(id))
		models = append(models, compiled)
	}
	if err := indexCompiledModelsRows(&c.rt, models, c.contentWork.spend); err != nil {
		return contentRestrictionCompileError(err)
	}
	c.rt.CompiledContent = compiledContent{Sources: sources, Models: models}
	return nil
}

func (c *compiler) compileContentModel(cc *contentModelCompiler, compiled *compiledModel, id contentModelID) (err error) {
	scope, err := c.enterContentModel(id)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	*compiled, err = cc.compileRootContentModel(id)
	return err
}

func (c *compiler) checkCompiledModelsUPA(reached []bool) error {
	upa := newContentModelUPACheck(&c.rt.Names, &c.rt, len(c.rt.Models), &c.contentWork, c.contentAnalysis)
	for id, ok := range reached {
		if !ok {
			continue
		}
		if err := c.checkModelUPA(upa, contentModelID(id)); err != nil {
			return err
		}
	}
	return nil
}

// complexTypeReachableModels returns the content models that Unique Particle
// Attribution and automaton construction apply to, as
// docs/spec/INTERPRETATIONS.md scopes them.
func (c *compiler) complexTypeReachableModels() ([]bool, error) {
	walk := modelReach{
		reached: make([]bool, len(c.rt.Models)),
		pending: make([]contentModelID, 0, len(c.rt.ComplexTypes)),
	}
	for i := range c.rt.ComplexTypes {
		if err := walk.reach(c.rt.ComplexTypes[i].Content); err != nil {
			return nil, err
		}
	}
	for len(walk.pending) > 0 {
		id := walk.pending[len(walk.pending)-1]
		walk.pending = walk.pending[:len(walk.pending)-1]
		if err := c.contentWork.spend(1); err != nil {
			return nil, err
		}
		if err := walk.reachParticles(c.rt.Models[id].Particles); err != nil {
			return nil, err
		}
	}
	return walk.reached, nil
}

type modelReach struct {
	reached []bool
	pending []contentModelID
}

func (r *modelReach) reach(id contentModelID) error {
	if !validContentModelID(id, len(r.reached)) {
		return xsderrors.InternalInvariant("content model reachability references missing content model")
	}
	if !r.reached[id] {
		r.reached[id] = true
		r.pending = append(r.pending, id)
	}
	return nil
}

func (r *modelReach) reachParticles(particles []contentParticle) error {
	for _, particle := range particles {
		if particle.Kind != particleModel {
			continue
		}
		if err := r.reach(particle.Model); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) checkModelUPA(upa *contentModelUPACheck, id contentModelID) (err error) {
	scope, err := c.enterContentModel(id)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	return upa.checkModel(id)
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
