package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) addModel(m runtime.ContentModel) (runtime.ContentModelID, error) {
	id, err := NextContentModelID(len(c.rt.Models))
	if err != nil {
		return runtime.NoContentModel, err
	}
	c.rt.Models = append(c.rt.Models, m)
	return id, nil
}

func (c *compiler) contentModel(id runtime.ContentModelID, msg string) (runtime.ContentModel, error) {
	if !runtime.ValidContentModelID(id, len(c.rt.Models)) {
		return runtime.ContentModel{}, xsderrors.InternalInvariant(msg)
	}
	return c.rt.Models[id], nil
}

func (c *compiler) compileModel(n *rawNode, ctx *schemaContext) (runtime.ContentModelID, error) {
	if n.Name.Local == vocab.XSDElemGroup {
		if ref, ok := n.attr(vocab.XSDAttrRef); ok {
			return c.compileModelGroupRef(n, ctx, ref)
		}
	}
	if id, ok := c.modelDone[n]; ok {
		if c.compilingModel[n] {
			if c.elementDepth > c.modelDepth[n] {
				return id, nil
			}
			err := CheckSchemaComponentRecursion(SchemaComponentModelGroup, true, "")
			return runtime.NoContentModel, withSchemaCompileLocation(n, err)
		}
		return id, nil
	}
	id, err := c.addModel(runtime.ContentModel{})
	if err != nil {
		return runtime.NoContentModel, err
	}
	c.modelDone[n] = id
	c.modelDepth[n] = c.elementDepth
	c.compilingModel[n] = true
	defer delete(c.compilingModel, n)
	kind, err := modelKindForNode(n)
	if err != nil {
		return runtime.NoContentModel, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return runtime.NoContentModel, err
	}
	if kind == runtime.ModelAll {
		if err := ValidateAllModelOccurrence(occurs); err != nil {
			return runtime.NoContentModel, withSchemaCompileLocation(n, err)
		}
	}
	m := runtime.ContentModel{Kind: kind, Occurs: occurs}
	if err := c.compileModelChildren(n, ctx, &m); err != nil {
		return runtime.NoContentModel, err
	}
	if err := c.checkElementDeclarationsConsistent(m); err != nil {
		return runtime.NoContentModel, withSchemaCompileLocation(n, err)
	}
	c.rt.Models[id] = m
	return id, nil
}

func (c *compiler) compileModelGroupRef(n *rawNode, ctx *schemaContext, ref string) (runtime.ContentModelID, error) {
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return runtime.NoContentModel, err
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return runtime.NoContentModel, err
	}
	label := c.rt.Names.Format(q)
	raw, ok := c.groupRaw[q]
	if existsErr := CheckSchemaComponentExists(SchemaComponentModelGroup, ok, label); existsErr != nil {
		return runtime.NoContentModel, withSchemaCompileLocation(n, existsErr)
	}
	modelNode, err := checkTopLevelGroupChildren(raw.node)
	if err != nil {
		return runtime.NoContentModel, err
	}
	if id, ok := c.modelDone[modelNode]; ok && c.compilingModel[modelNode] {
		return c.recursiveModelGroupRef(q, id, occurs, modelNode)
	}
	id, err := c.compileModel(modelNode, raw.ctx)
	if err != nil {
		return runtime.NoContentModel, err
	}
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
	return c.addModel(model)
}

func (c *compiler) recursiveModelGroupRef(q runtime.QName, id runtime.ContentModelID, occurs runtime.Occurrence, modelNode *rawNode) (runtime.ContentModelID, error) {
	if c.elementDepth <= c.modelDepth[modelNode] {
		err := CheckSchemaComponentRecursion(SchemaComponentModelGroup, true, c.rt.Names.Format(q))
		return runtime.NoContentModel, withSchemaCompileLocation(modelNode, err)
	}
	ref := runtime.ContentModel{
		Kind:      runtime.ModelSequence,
		Occurs:    occurs,
		Particles: []runtime.Particle{runtime.ModelParticle(id, runtime.Occurrence{Min: 1, Max: 1})},
	}
	return c.addModel(ref)
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
		if child.Name.Space != runtime.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
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
	if admissionErr := validateModelGroupChildAdmission(child, m.Kind, admission); admissionErr != nil {
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
	if admissionErr := validateModelGroupChildAdmission(child, m.Kind, ModelChildAdmissionForModelKind(childModel.Kind)); admissionErr != nil {
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

func validateModelGroupChildAdmission(n *rawNode, parent runtime.ModelKind, child ModelChildAdmission) error {
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
		modelID, err = c.addModel(normalized)
		if err != nil {
			return runtime.Particle{}, false, err
		}
	}
	return runtime.ModelParticle(modelID, occurs), true, nil
}

func (c *compiler) validateComplexExtensionModelAdmission(baseID runtime.ComplexTypeID, base runtime.ComplexType, ext runtime.ContentModelID, mixed bool) error {
	modelRT := newContentModelCompiler(&c.rt.Names, &c.rt, c.limits.MaxContentModelStates)
	return ValidateComplexExtensionModelAdmission(&modelRT, ComplexExtensionModelAdmission{
		Extension:     ext,
		BaseContent:   base.Content,
		BaseIsAnyType: baseID == c.rt.Builtin.AnyType,
		BaseMixed:     base.Mixed(),
		Mixed:         mixed,
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
	models, err := CompileContentModels(
		&c.rt.Names,
		&c.rt,
		len(c.rt.Models),
		c.limits.MaxContentModelStates,
	)
	if err != nil {
		return err
	}
	c.rt.CompiledModels = models
	return nil
}

func (c *compiler) checkCompiledModelsUPA() error {
	return CheckContentModelsUPA(&c.rt.Names, &c.rt, len(c.rt.Models))
}

func (c *compiler) checkCompiledElementDeclarationsConsistent() error {
	return CheckContentModelElementDeclarationsConsistent(elementDeclarationModelRuntime{rt: &c.rt, models: c.rt.Models}, len(c.rt.Models))
}

func (c *compiler) checkElementDeclarationsConsistent(model runtime.ContentModel) error {
	return CheckElementDeclarationsConsistent(elementDeclarationModelRuntime{rt: &c.rt, models: c.rt.Models}, model)
}
