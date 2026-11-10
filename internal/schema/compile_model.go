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

func (c *compiler) compileModel(n *schemaNode, ctx *schemaSetDocument) (contentModelID, error) {
	if source := semanticSource[*schemaGroupSource](&n.semantic); source != nil && source.Ref.Lexical.Present {
		return c.compileModelGroupRef(n, ctx, source.Ref.Name)
	}
	if id, done, err := c.existingModel(n, ctx); done || err != nil {
		return id, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return noContentModel, err
	}
	if (n.local == vocab.XSDElemSequence || n.local == vocab.XSDElemChoice) && occurs.Max == 0 && !occurs.Unbounded {
		if err := c.spendComponentDependency(n); err != nil {
			return noContentModel, err
		}
		// Cache only the parent's absence. Its descendants retain their own
		// obligations after active group expansion unwinds.
		c.modelDone[nodeKey(n, ctx)] = noContentModel
		c.deferredModelChildren = append(c.deferredModelChildren, schemaComponent{ctx: ctx, id: n.id})
		return noContentModel, nil
	}
	return c.compileModelComponent(n, ctx, occurs)
}

func (c *compiler) compileModelComponent(n *schemaNode, ctx *schemaSetDocument, occurs occurrence) (result contentModelID, err error) {
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return noContentModel, dependencyErr
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return noContentModel, err
	}
	defer leave(&err)
	id, err := c.addModelAt(contentModelNode{}, n)
	if err != nil {
		return noContentModel, err
	}
	c.modelDone[nodeKey(n, ctx)] = id
	c.compilingModel[nodeKey(n, ctx)] = true
	defer delete(c.compilingModel, nodeKey(n, ctx))
	m, err := c.compileModelValue(n, ctx, occurs)
	if err != nil {
		return noContentModel, err
	}
	c.completeModel(id, m)
	return id, nil
}

func (c *compiler) existingModel(n *schemaNode, ctx *schemaSetDocument) (contentModelID, bool, error) {
	id, ok := c.modelDone[nodeKey(n, ctx)]
	if !ok {
		return noContentModel, false, nil
	}
	if !c.compilingModel[nodeKey(n, ctx)] {
		return id, true, nil
	}
	err := schemaComponentRecursionError(schemaComponentModelGroup, "")
	return noContentModel, true, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileModelValue(n *schemaNode, ctx *schemaSetDocument, occurs occurrence) (contentModelNode, error) {
	kind, err := modelKindForNode(n)
	if err != nil {
		return contentModelNode{}, err
	}
	if kind == modelAll {
		if err := validateAllModelOccurrence(occurs); err != nil {
			return contentModelNode{}, withSchemaCompileLocation(n, err)
		}
	}
	m := contentModelNode{Kind: kind, Occurs: occurs}
	if err := c.compileModelChildren(n, ctx, &m); err != nil {
		return contentModelNode{}, err
	}
	return m, nil
}

func (c *compiler) compileModelGroupRef(n *schemaNode, ctx *schemaSetDocument, ref xml.Name) (contentModelID, error) {
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return noContentModel, err
	}
	// A zero-occurrence reference has no particle and therefore no term to resolve.
	if occurs.Max == 0 && !occurs.Unbounded {
		return noContentModel, nil
	}
	source, err := c.resolveModelGroupRef(n, ctx, ref)
	if err != nil {
		return noContentModel, err
	}
	if c.compilingModel[nodeKey(source.modelNode, source.raw.ctx)] {
		err = schemaComponentRecursionError(schemaComponentModelGroup, c.rt.formatName(source.q))
		return noContentModel, withSchemaCompileLocation(source.modelNode, err)
	}
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return noContentModel, dependencyErr
	}
	id, err := c.compileModel(source.modelNode, source.raw.ctx)
	if err != nil {
		return noContentModel, err
	}
	return c.applyModelGroupOccurrence(n, id, occurs)
}

type modelGroupRefSource struct {
	modelNode *schemaNode
	raw       schemaComponent
	q         QName
}

func (c *compiler) resolveModelGroupRef(n *schemaNode, ctx *schemaSetDocument, ref xml.Name) (modelGroupRefSource, error) {
	q, err := c.referenceQName(n, ctx, ref)
	if err != nil {
		return modelGroupRefSource{}, err
	}
	label := c.rt.formatName(q)
	raw, ok := c.groupComponents[q]
	if !ok {
		return modelGroupRefSource{}, withSchemaCompileLocation(n, schemaComponentMissingError(schemaComponentModelGroup, label))
	}
	modelNode, err := checkTopLevelGroupChildren(raw.sourceNode())
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
	return c.addModelAt(model, n)
}

func modelKindForNode(n *schemaNode) (modelKind, error) {
	if kind, ok := schemaModelKind(n); ok {
		return kind, nil
	}
	return 0, withSchemaCompileLocation(n, xsderrors.InternalInvariant("model node has no typed model kind"))
}

func (c *compiler) compileModelChildren(n *schemaNode, ctx *schemaSetDocument, m *contentModelNode) error {
	for child := range n.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		if err := c.appendModelChild(m, child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) appendModelChild(m *contentModelNode, child *schemaNode, ctx *schemaSetDocument) error {
	switch child.local {
	case vocab.XSDElemElement:
		p, err := c.compileElementParticle(child, ctx)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, m, p))
	case vocab.XSDElemAny:
		p, err := c.compileWildcardParticle(child, ctx)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, appendParticle(&c.dependencyWork, m, p))
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		return c.appendNestedModelChild(m, child, ctx)
	}
	return withSchemaCompileLocation(child, xsderrors.InternalInvariant("model child has no supported typed schema kind"))
}

func (c *compiler) compileNestedModelChild(child *schemaNode, ctx *schemaSetDocument, parent modelKind) (contentModelID, contentModelNode, error) {
	admission, err := modelChildAdmissionForNode(child)
	if err != nil {
		return noContentModel, contentModelNode{}, err
	}
	if admissionErr := validateModelGroupChildAtNode(child, parent, admission); admissionErr != nil {
		return noContentModel, contentModelNode{}, admissionErr
	}
	id, err := c.compileModel(child, ctx)
	if err != nil || id == noContentModel {
		return id, contentModelNode{}, err
	}
	model, err := c.contentModel(id, "nested model child references missing content model")
	if err != nil {
		return noContentModel, contentModelNode{}, err
	}
	if err := validateModelGroupChildAtNode(child, parent, modelChildAdmissionForModelKind(model.Kind)); err != nil {
		return noContentModel, contentModelNode{}, err
	}
	return id, model, nil
}

func (c *compiler) appendNestedModelChild(m *contentModelNode, child *schemaNode, ctx *schemaSetDocument) error {
	childModelID, childModel, err := c.compileNestedModelChild(child, ctx, m.Kind)
	if err != nil || childModelID == noContentModel {
		return err
	}
	flattened, err := appendFlattenedModelChild(&c.dependencyWork, m, childModel)
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	if flattened {
		return nil
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
			_, err = c.compileElementParticle(child, source.ctx)
		case vocab.XSDElemAny:
			_, err = c.compileWildcardParticle(child, source.ctx)
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
			_, _, err = c.compileNestedModelChild(child, source.ctx, kind)
		default:
			return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "invalid model group child "+diagnostictext.Quote(child.local))
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

func modelChildAdmissionForNode(n *schemaNode) (modelChildAdmission, error) {
	switch n.local {
	case vocab.XSDElemElement:
		return modelChildAdmission{Kind: modelChildElement}, nil
	case vocab.XSDElemAny:
		return modelChildAdmission{Kind: modelChildWildcard}, nil
	case vocab.XSDElemGroup:
		return modelChildAdmission{Kind: modelChildModel}, nil
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
		kind, ok := schemaModelKind(n)
		if !ok {
			return modelChildAdmission{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("model group node has no typed model kind"))
		}
		return modelChildAdmissionForModelKind(kind), nil
	}
	return modelChildAdmission{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("model child has no typed schema kind"))
}

func validateModelOccurrence(n *schemaNode, limits Limits) error {
	if n.local == vocab.XSDElemGroup {
		if err := checkGroupOccurrenceAttributes(n); err != nil {
			return err
		}
	}
	occurs, err := parseOccurs(n, limits)
	if err != nil {
		return err
	}
	if n.local == vocab.XSDElemAll {
		if err := validateAllModelOccurrence(occurs); err != nil {
			return withSchemaCompileLocation(n, err)
		}
	}
	return validateModelGroupSyntax(n, limits)
}

func (c *compiler) modelParticle(id contentModelID) (contentParticle, bool, error) {
	model, err := c.contentModel(id, "model particle references missing content model")
	if err != nil {
		return contentParticle{}, false, err
	}
	occurs := model.Occurs
	if occurs.Max == 0 && !occurs.Unbounded {
		return contentParticle{}, false, nil
	}
	modelID := id
	if !occurs.IsExactlyOne() {
		normalized := model
		normalized.Occurs = occurrence{Min: 1, Max: 1}
		var err error
		modelID, err = c.addModelAt(normalized, c.modelSources[id])
		if err != nil {
			return contentParticle{}, false, err
		}
	}
	return modelParticle(modelID, occurs), true, nil
}

func (c *compiler) validateComplexExtensionModelAdmission(baseID ComplexTypeID, base complexTypeMetadata, ext contentModelID, contentKind complexContentVariety) error {
	return validateComplexExtensionModelAdmission(&c.rt, complexExtensionModelAdmission{
		Extension:     ext,
		BaseContent:   base.Content,
		BaseIsAnyType: baseID == c.rt.AnyType,
		BaseMixed:     base.Mixed(),
		Mixed:         contentKind.Mixed(),
	})
}

func parseOccurs(n *schemaNode, limits Limits) (occurrence, error) {
	attrs, err := occurrenceAttrs(n)
	if err != nil {
		return occurrence{}, withSchemaCompileLocation(n, err)
	}
	occurs, err := parseOccurrence(attrs, limits)
	if err != nil {
		return occurrence{}, withSchemaCompileLocation(n, err)
	}
	return occurs, nil
}

func occurrenceAttrs(n *schemaNode) (rawOccurrence, error) {
	source := n.semantic.particle()
	if source == nil {
		return rawOccurrence{}, xsderrors.InternalInvariant("particle node has no typed particle source")
	}
	return rawOccurrence{
		MinOccurs:    source.MinOccurs.Value,
		MaxOccurs:    source.MaxOccurs.Value,
		HasMinOccurs: source.MinOccurs.Present,
		HasMaxOccurs: source.MaxOccurs.Present,
	}, nil
}

func (c *compiler) compileContentModels() error {
	models, err := c.compileContentModelsBuild()
	if err != nil {
		return err
	}
	return c.installCompiledModels(models)
}

func (c *compiler) checkCompiledModelsUPA() error {
	return c.checkContentModelsUPABuild()
}

func (c *compiler) checkCompiledElementDeclarationsConsistent() error {
	return c.checkContentModelElementDeclarationsConsistentBuild()
}
