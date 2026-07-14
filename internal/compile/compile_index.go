package compile

import "github.com/jacoelho/xsd/internal/vocab"

func (c *compiler) index() error {
	for _, document := range c.schemas.documents {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
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
	ctx := c.schemaContext(document)
	c.contexts[doc] = ctx
	for child := range doc.root.xsdChildren() {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if err := c.indexTopLevelSchemaChild(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) schemaContext(document schemaSetDocument) *schemaContext {
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
		return withSchemaCompileLocation(child, c.indexGlobalAttribute(q, component, label))
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
			if err := checkGroupOccurrenceAttributes(child); err != nil {
				return err
			}
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
