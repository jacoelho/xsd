package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) contentModel(id runtime.ContentModelID, msg string) (runtime.ContentModel, error) {
	model, ok := c.rt.ContentModel(id)
	if !ok {
		return runtime.ContentModel{}, xsderrors.InternalInvariant(msg)
	}
	return model, nil
}

func (c *compiler) compileModel(n *rawNode, ctx *schemaContext) (runtime.ContentModelID, error) {
	if n.Name.Local == vocab.XSDElemGroup {
		if ref, ok := n.attr(vocab.XSDAttrRef); ok {
			return c.compileModelGroupRef(n, ctx, ref)
		}
	}
	if id, done, err := c.existingModel(n); done || err != nil {
		return id, err
	}
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return runtime.NoContentModel, dependencyErr
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return runtime.NoContentModel, err
	}
	defer leave()
	id, err := c.addModelAt(runtime.ContentModel{}, n)
	if err != nil {
		return runtime.NoContentModel, err
	}
	c.modelDone[n] = id
	c.modelDepth[n] = c.elementDepth
	c.compilingModel[n] = true
	defer delete(c.compilingModel, n)
	m, err := c.compileModelValue(n, ctx)
	if err != nil {
		return runtime.NoContentModel, err
	}
	c.completeModel(id, m)
	return id, nil
}

func (c *compiler) existingModel(n *rawNode) (runtime.ContentModelID, bool, error) {
	id, ok := c.modelDone[n]
	if !ok {
		return runtime.NoContentModel, false, nil
	}
	if !c.compilingModel[n] || c.elementDepth > c.modelDepth[n] {
		return id, true, nil
	}
	err := SchemaComponentRecursionError(SchemaComponentModelGroup, "")
	return runtime.NoContentModel, true, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileModelValue(n *rawNode, ctx *schemaContext) (runtime.ContentModel, error) {
	kind, err := modelKindForNode(n)
	if err != nil {
		return runtime.ContentModel{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return runtime.ContentModel{}, err
	}
	if kind == runtime.ModelAll {
		if err := ValidateAllModelOccurrence(occurs); err != nil {
			return runtime.ContentModel{}, withSchemaCompileLocation(n, err)
		}
	}
	m := runtime.ContentModel{Kind: kind, Occurs: occurs}
	if err := c.compileModelChildren(n, ctx, &m); err != nil {
		return runtime.ContentModel{}, err
	}
	return m, nil
}

func (c *compiler) compileModelGroupRef(n *rawNode, ctx *schemaContext, ref string) (runtime.ContentModelID, error) {
	source, err := c.resolveModelGroupRef(n, ctx, ref)
	if err != nil {
		return runtime.NoContentModel, err
	}
	if id, exists := c.modelDone[source.modelNode]; exists && c.compilingModel[source.modelNode] {
		return c.compileRecursiveModelGroupRef(n, source, id)
	}
	if dependencyErr := c.spendComponentDependency(n); dependencyErr != nil {
		return runtime.NoContentModel, dependencyErr
	}
	id, err := c.compileModel(source.modelNode, source.raw.ctx)
	if err != nil {
		return runtime.NoContentModel, err
	}
	return c.applyModelGroupOccurrence(n, id, source.occurs)
}

type modelGroupRefSource struct {
	raw       rawComponent
	modelNode *rawNode
	occurs    runtime.Occurrence
	q         runtime.QName
}

func (c *compiler) resolveModelGroupRef(n *rawNode, ctx *schemaContext, ref string) (modelGroupRefSource, error) {
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return modelGroupRefSource{}, err
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return modelGroupRefSource{}, err
	}
	label := c.rt.formatName(q)
	raw, ok := c.groupRaw[q]
	if !ok {
		return modelGroupRefSource{}, withSchemaCompileLocation(n, SchemaComponentMissingError(SchemaComponentModelGroup, label))
	}
	modelNode, err := checkTopLevelGroupChildren(raw.node)
	if err != nil {
		return modelGroupRefSource{}, err
	}
	return modelGroupRefSource{raw: raw, q: q, modelNode: modelNode, occurs: occurs}, nil
}

func (c *compiler) compileRecursiveModelGroupRef(n *rawNode, source modelGroupRefSource, id runtime.ContentModelID) (runtime.ContentModelID, error) {
	if c.elementDepth > c.modelDepth[source.modelNode] {
		if err := c.spendComponentDependency(n); err != nil {
			return runtime.NoContentModel, err
		}
	}
	return c.recursiveModelGroupRef(source.q, id, source.occurs, source.modelNode)
}

func (c *compiler) applyModelGroupOccurrence(n *rawNode, id runtime.ContentModelID, occurs runtime.Occurrence) (runtime.ContentModelID, error) {
	if occurs.IsExactlyOne() {
		return id, nil
	}
	model, err := c.contentModel(id, "model group reference resolved missing content model")
	if err != nil {
		return runtime.NoContentModel, err
	}
	if model.Kind == runtime.ModelAll {
		if err := ValidateAllModelOccurrence(occurs); err != nil {
			return runtime.NoContentModel, withSchemaCompileLocation(n, err)
		}
	}
	model.Occurs = occurs
	return c.addModelAt(model, n)
}

func (c *compiler) recursiveModelGroupRef(q runtime.QName, id runtime.ContentModelID, occurs runtime.Occurrence, modelNode *rawNode) (runtime.ContentModelID, error) {
	if c.elementDepth <= c.modelDepth[modelNode] {
		err := SchemaComponentRecursionError(SchemaComponentModelGroup, c.rt.formatName(q))
		return runtime.NoContentModel, withSchemaCompileLocation(modelNode, err)
	}
	ref := runtime.ContentModel{
		Kind:      runtime.ModelSequence,
		Occurs:    occurs,
		Particles: []runtime.Particle{runtime.ModelParticle(id, runtime.Occurrence{Min: 1, Max: 1})},
	}
	return c.addModelAt(ref, modelNode)
}

func modelKindForNode(n *rawNode) (runtime.ModelKind, error) {
	kind, err := ModelKindForLocal(n.Name.Local)
	if err != nil {
		return 0, withSchemaCompileLocation(n, err)
	}
	return kind, nil
}

func (c *compiler) compileModelChildren(n *rawNode, ctx *schemaContext, m *runtime.ContentModel) error {
	for _, child := range n.Children {
		if child.Name.Space != vocab.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		if err := c.appendModelChild(m, child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) appendModelChild(m *runtime.ContentModel, child *rawNode, ctx *schemaContext) error {
	switch child.Name.Local {
	case vocab.XSDElemElement:
		p, err := c.compileElementParticle(child, ctx)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, AppendParticle(m, p))
	case vocab.XSDElemAny:
		p, err := c.compileWildcardParticle(child, ctx)
		if err != nil {
			return err
		}
		return withSchemaCompileLocation(child, AppendParticle(m, p))
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		return c.appendNestedModelChild(m, child, ctx)
	default:
		return nil
	}
}

func (c *compiler) appendNestedModelChild(m *runtime.ContentModel, child *rawNode, ctx *schemaContext) error {
	admission, err := modelChildAdmissionForNode(child)
	if err != nil {
		return err
	}
	if admissionErr := validateModelGroupChildAtNode(child, m.Kind, admission); admissionErr != nil {
		return admissionErr
	}
	childModelID, err := c.compileModel(child, ctx)
	if err != nil {
		return err
	}
	childModel, err := c.contentModel(childModelID, "nested model child references missing content model")
	if err != nil {
		return err
	}
	if admissionErr := validateModelGroupChildAtNode(child, m.Kind, ModelChildAdmissionForModelKind(childModel.Kind)); admissionErr != nil {
		return admissionErr
	}
	if AppendFlattenedModelChild(m, childModel) {
		return nil
	}
	p, ok, err := c.modelParticle(childModelID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return withSchemaCompileLocation(child, AppendParticle(m, p))
}

func validateModelGroupChildAtNode(n *rawNode, parent runtime.ModelKind, child ModelChildAdmission) error {
	return withSchemaCompileLocation(n, ValidateModelGroupChildAdmission(parent, child))
}

func modelChildAdmissionForNode(n *rawNode) (ModelChildAdmission, error) {
	admission, err := ModelChildAdmissionForLocal(n.Name.Local)
	if err != nil {
		return ModelChildAdmission{}, withSchemaCompileLocation(n, err)
	}
	return admission, nil
}

func validateModelOccurrence(n *rawNode, limits Limits) error {
	if n.Name.Local == vocab.XSDElemGroup {
		if err := checkGroupOccurrenceAttributes(n); err != nil {
			return err
		}
	}
	occurs, err := parseOccurs(n, limits)
	if err != nil {
		return err
	}
	if n.Name.Local == vocab.XSDElemAll {
		if err := ValidateAllModelOccurrence(occurs); err != nil {
			return withSchemaCompileLocation(n, err)
		}
	}
	return validateRawModelGroupSyntax(n, limits)
}

func (c *compiler) modelParticle(id runtime.ContentModelID) (runtime.Particle, bool, error) {
	model, err := c.contentModel(id, "model particle references missing content model")
	if err != nil {
		return runtime.Particle{}, false, err
	}
	occurs := model.Occurs
	if occurs.Max == 0 && !occurs.Unbounded {
		return runtime.Particle{}, false, nil
	}
	modelID := id
	if !occurs.IsExactlyOne() {
		normalized := model
		normalized.Occurs = runtime.Occurrence{Min: 1, Max: 1}
		var err error
		modelID, err = c.addModelAt(normalized, c.modelSources[id])
		if err != nil {
			return runtime.Particle{}, false, err
		}
	}
	return runtime.ModelParticle(modelID, occurs), true, nil
}

func (c *compiler) validateComplexExtensionModelAdmission(baseID runtime.ComplexTypeID, base runtime.ComplexType, ext runtime.ContentModelID, contentKind runtime.ContentKind) error {
	return ValidateComplexExtensionModelAdmission(&c.rt, ComplexExtensionModelAdmission{
		Extension:     ext,
		BaseContent:   base.Content,
		BaseIsAnyType: baseID == c.rt.builtinIDs().AnyType,
		BaseMixed:     base.Mixed(),
		Mixed:         contentKind.Mixed(),
	})
}

func parseOccurs(n *rawNode, limits Limits) (runtime.Occurrence, error) {
	occurs, err := ParseOccurrence(occurrenceAttrs(n), limits)
	if err != nil {
		return runtime.Occurrence{}, withSchemaCompileLocation(n, err)
	}
	return occurs, nil
}

func occurrenceAttrs(n *rawNode) OccurrenceAttrs {
	minOccurs, hasMinOccurs := n.attr(vocab.XSDAttrMinOccurs)
	maxOccurs, hasMaxOccurs := n.attr(vocab.XSDAttrMaxOccurs)
	return OccurrenceAttrs{
		MinOccurs:    minOccurs,
		MaxOccurs:    maxOccurs,
		HasMinOccurs: hasMinOccurs,
		HasMaxOccurs: hasMaxOccurs,
	}
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
