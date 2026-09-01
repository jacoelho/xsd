package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

func (c *compiler) index() error {
	for _, document := range c.plan.documents {
		if !document.indexDeclarations {
			continue
		}
		if err := c.indexSchemaDocument(document); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) indexSchemaDocument(document schemaSetDocument) error {
	doc := document.doc
	ctx := newSchemaContext(document)
	c.contexts[doc] = ctx
	for child := range doc.root.xsdChildren() {
		if err := c.indexTopLevelSchemaChild(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func newSchemaContext(document schemaSetDocument) *schemaContext {
	doc := document.doc
	defaults := doc.defaults
	ctx := &schemaContext{
		doc:              doc,
		targetNS:         defaults.TargetNamespace,
		elementQualified: defaults.ElementQualified,
		attrQualified:    defaults.AttributeQualified,
		blockDefault:     defaults.BlockDefault,
		finalDefault:     defaults.FinalDefault,
		imports:          document.imports,
		adoptedTarget:    document.adoptedTarget,
	}
	if ctx.targetNS == "" {
		ctx.targetNS = document.effectiveTargetNS
	}
	return ctx
}

func (c *compiler) indexTopLevelSchemaChild(child *rawNode, ctx *schemaContext) error {
	if err := c.validateTopLevelSchemaChild(child, ctx); err != nil {
		return err
	}
	name, ok := child.attr(vocab.XSDAttrName)
	if !ok {
		return nil
	}
	q, err := c.rt.internQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	label := c.rt.formatName(q)
	component := rawComponent{child, ctx}
	return c.indexNamedTopLevelSchemaChild(child, q, label, component)
}

func (c *compiler) indexNamedTopLevelSchemaChild(child *rawNode, q runtime.QName, label string, component rawComponent) error {
	switch child.Name.Local {
	case vocab.XSDElemSimpleType:
		return c.indexType(child, q, label, component, c.simpleRaw)
	case vocab.XSDElemComplexType:
		return c.indexType(child, q, label, component, c.complexRaw)
	case vocab.XSDElemElement:
		return withSchemaCompileLocation(child, AddSchemaComponent(c.elementRaw, q, component, label))
	case vocab.XSDElemAttribute:
		return withSchemaCompileLocation(child, c.indexGlobalAttribute(q, component, label))
	case vocab.XSDElemGroup:
		return c.indexModelGroup(child, q, label, component)
	case vocab.XSDElemAttributeGroup:
		return withSchemaCompileLocation(child, AddSchemaComponent(c.attrGroupRaw, q, component, label))
	default:
		return nil
	}
}

func (c *compiler) indexType(child *rawNode, q runtime.QName, label string, component rawComponent, dst map[runtime.QName]rawComponent) error {
	if _, exists := dst[q]; exists {
		return withSchemaCompileLocation(child, AddSchemaComponent(dst, q, component, label))
	}
	if c.typeQNameKnown(q) {
		return withSchemaCompileLocation(child, SchemaTypeNameConflictError(label))
	}
	return withSchemaCompileLocation(child, AddSchemaComponent(dst, q, component, label))
}

func (c *compiler) indexModelGroup(child *rawNode, q runtime.QName, label string, component rawComponent) error {
	model, err := checkTopLevelGroupChildren(child)
	if err != nil {
		return err
	}
	if err := validateRawModelGroupSyntax(model, c.limits); err != nil {
		return err
	}
	return withSchemaCompileLocation(child, AddSchemaComponent(c.groupRaw, q, component, label))
}

func (c *compiler) validateTopLevelSchemaChild(child *rawNode, ctx *schemaContext) error {
	if err := checkTopLevelSchemaChild(child); err != nil {
		return err
	}
	if child.Name.Local == vocab.XSDElemNotation {
		return c.indexNotation(child, ctx)
	}
	return nil
}

func (c *compiler) indexNotation(n *rawNode, ctx *schemaContext) error {
	if err := checkNotationDeclaration(n); err != nil {
		return err
	}
	name, _ := n.attr(vocab.XSDAttrName)
	q, err := c.rt.internQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	return withSchemaCompileLocation(n, c.addNotation(q, c.rt.formatName(q)))
}

func validateRawModelGroupSyntax(n *rawNode, limits Limits) error {
	if n.Name.Local == vocab.XSDElemGroup {
		return checkChildOrderRules(n, groupUseChildOrder)
	}
	parentKind, err := ModelKindForLocal(n.Name.Local)
	if err != nil {
		return withSchemaCompileLocation(n, err)
	}
	if err := checkChildOrderRules(n, modelGroupChildOrder(n.Name.Local)); err != nil {
		return err
	}
	for child := range n.xsdChildren() {
		if child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		if err := validateRawModelGroupChild(child, parentKind, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateRawModelGroupChild(child *rawNode, parent runtime.ModelKind, limits Limits) error {
	admission, err := ModelChildAdmissionForLocal(child.Name.Local)
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	if err := ValidateModelGroupChildAdmission(parent, admission); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	switch child.Name.Local {
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
		return validateNestedRawModelGroupOccurrence(child, limits)
	case vocab.XSDElemGroup:
		return validateRawModelGroupReference(child)
	case vocab.XSDElemAny:
		return checkChildOrderRules(child, anyParticleChildOrder)
	default:
		return nil
	}
}

func validateRawModelGroupReference(child *rawNode) error {
	if err := checkGroupOccurrenceAttributes(child); err != nil {
		return err
	}
	if err := ValidateGroupUseSource(rawLexicalAttribute(child, vocab.XSDAttrRef)); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return checkChildOrderRules(child, groupUseChildOrder)
}

func validateNestedRawModelGroupOccurrence(n *rawNode, limits Limits) error {
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
