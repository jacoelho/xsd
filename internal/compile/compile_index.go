package compile

import "github.com/jacoelho/xsd/internal/vocab"

func (c *compiler) index() error {
	for _, doc := range c.compileDocs {
		if err := c.indexSchemaDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) indexSchemaDocument(doc *rawDoc) error {
	ctx, err := c.schemaContext(doc)
	if err != nil {
		return err
	}
	c.contexts[doc] = ctx
	for child := range doc.root.xsdChildren() {
		if err := c.indexTopLevelSchemaChild(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) schemaContext(doc *rawDoc) (*schemaContext, error) {
	root := doc.root
	defaults, err := parseSchemaDefaults(root)
	if err != nil {
		return nil, err
	}
	ctx := &schemaContext{
		doc:              doc,
		targetNS:         defaults.TargetNamespace,
		elementQualified: defaults.ElementQualified,
		attrQualified:    defaults.AttributeQualified,
		blockDefault:     defaults.BlockDefault,
		finalDefault:     defaults.FinalDefault,
		imports:          c.imports[doc.key],
	}
	if ctx.targetNS == "" && c.adoptTarget[doc.key] != "" {
		ctx.targetNS = c.adoptTarget[doc.key]
	}
	return ctx, nil
}

func parseSchemaDefaults(root *rawNode) (SchemaDefaults, error) {
	target, hasTarget := root.attr(vocab.XSDAttrTargetNamespace)
	elementForm, hasElementForm := root.attr(vocab.XSDAttrElementFormDefault)
	attributeForm, hasAttributeForm := root.attr(vocab.XSDAttrAttributeFormDefault)
	defaults, err := ParseSchemaDefaults(SchemaDefaultAttrs{
		TargetNamespace:         target,
		BlockDefault:            root.attrValue(vocab.XSDAttrBlockDefault),
		FinalDefault:            root.attrValue(vocab.XSDAttrFinalDefault),
		ElementFormDefault:      elementForm,
		AttributeFormDefault:    attributeForm,
		HasTargetNamespace:      hasTarget,
		HasElementFormDefault:   hasElementForm,
		HasAttributeFormDefault: hasAttributeForm,
	})
	return defaults, withSchemaCompileLocation(root, err)
}

func (c *compiler) indexTopLevelSchemaChild(child *rawNode, ctx *schemaContext) error {
	if err := c.validateTopLevelSchemaChild(child, ctx); err != nil {
		return err
	}
	name, ok := child.attr(vocab.XSDAttrName)
	if !ok {
		return nil
	}
	q, err := c.names.InternQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	label := c.rt.Names.Format(q)
	component := rawComponent{child, ctx}
	switch child.Name.Local {
	case vocab.XSDElemSimpleType:
		_, exists := c.complexRaw[q]
		if err := CheckSchemaTypeNameAvailable(exists, label); err != nil {
			return withSchemaCompileLocation(child, err)
		}
		return withSchemaCompileLocation(child, AddSchemaComponent(c.simpleRaw, q, component, label))
	case vocab.XSDElemComplexType:
		_, exists := c.simpleRaw[q]
		if err := CheckSchemaTypeNameAvailable(exists, label); err != nil {
			return withSchemaCompileLocation(child, err)
		}
		return withSchemaCompileLocation(child, AddSchemaComponent(c.complexRaw, q, component, label))
	case vocab.XSDElemElement:
		return withSchemaCompileLocation(child, AddSchemaComponent(c.elementRaw, q, component, label))
	case vocab.XSDElemAttribute:
		return withSchemaCompileLocation(child, AddGlobalAttributeComponent(c.attributeRaw, c.rt.GlobalAttributes, q, component, label))
	case vocab.XSDElemGroup:
		model, err := checkTopLevelGroupChildren(child)
		if err != nil {
			return err
		}
		if err := validateRawModelGroupSyntax(model, c.limits); err != nil {
			return err
		}
		return withSchemaCompileLocation(child, AddSchemaComponent(c.groupRaw, q, component, label))
	case vocab.XSDElemAttributeGroup:
		return withSchemaCompileLocation(child, AddSchemaComponent(c.attrGroupRaw, q, component, label))
	default:
		return nil
	}
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
	q, err := c.names.InternQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	return withSchemaCompileLocation(n, AddNotation(c.rt.Notations, q, c.rt.Names.Format(q)))
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
		admission, err := ModelChildAdmissionForLocal(child.Name.Local)
		if err != nil {
			return withSchemaCompileLocation(child, err)
		}
		if err := ValidateModelGroupChildAdmission(parentKind, admission); err != nil {
			return withSchemaCompileLocation(child, err)
		}
		switch child.Name.Local {
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
			if err := validateNestedRawModelGroupOccurrence(child, limits); err != nil {
				return err
			}
		case vocab.XSDElemGroup:
			_, hasRef := child.attr(vocab.XSDAttrRef)
			if err := ValidateGroupUseSource(hasRef); err != nil {
				return withSchemaCompileLocation(child, err)
			}
			if err := checkChildOrderRules(child, groupUseChildOrder); err != nil {
				return err
			}
		case vocab.XSDElemAny:
			if err := checkChildOrderRules(child, anyParticleChildOrder); err != nil {
				return err
			}
		}
	}
	return nil
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
