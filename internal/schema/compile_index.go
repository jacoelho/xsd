package schema

import (
	"github.com/jacoelho/xsd/internal/vocab"
)

// index scans each plan document once per effective-namespace context, so a
// chameleon document is rescanned for every including context. The repetition
// is bounded by the admitted target-context and instantiated-node limits.
func (c *compiler) index() error {
	for i := range c.plan.documents {
		document := &c.plan.documents[i]
		if err := c.indexSchemaDocument(document); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) indexSchemaDocument(ctx *schemaSetDocument) error {
	doc := ctx.doc
	for child := range doc.root.xsdChildren() {
		name, global := schemaGlobalName(&child.semantic)
		if !global {
			continue
		}
		if err := c.indexTopLevelSchemaDeclaration(ctx, name, child); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) indexTopLevelSchemaDeclaration(ctx *schemaSetDocument, name lexicalAttribute, child *schemaNode) error {
	// Top-level shape and declaration attributes were validated before the
	// immutable source node was published. Indexing only performs notation's
	// compiler-owned child check and registers the admitted declaration.
	if child.local == vocab.XSDElemNotation {
		return c.indexNotation(ctx, child, name.Value)
	}
	if !name.Present {
		return nil
	}
	q, err := c.rt.internQName(ctx.effectiveTargetNS, name.Value)
	if err != nil {
		return err
	}
	label := c.rt.formatName(q)
	component := schemaComponent{id: child.id, ctx: ctx}
	return c.indexNamedTopLevelSchemaChild(child, q, label, component)
}

func (c *compiler) indexNamedTopLevelSchemaChild(child *schemaNode, q QName, label string, component schemaComponent) error {
	switch child.local {
	case vocab.XSDElemSimpleType:
		return c.indexType(child, q, label, component, c.simpleComponents)
	case vocab.XSDElemComplexType:
		return c.indexType(child, q, label, component, c.complexComponents)
	case vocab.XSDElemElement:
		return withSchemaCompileLocation(child, addSchemaComponent(c.elementComponents, q, component, label))
	case vocab.XSDElemAttribute:
		return withSchemaCompileLocation(child, c.indexGlobalAttribute(q, component, label))
	case vocab.XSDElemGroup:
		return c.indexModelGroup(child, q, label, component)
	case vocab.XSDElemAttributeGroup:
		return withSchemaCompileLocation(child, addSchemaComponent(c.attributeGroupComponents, q, component, label))
	}
	return nil
}

func (c *compiler) indexType(child *schemaNode, q QName, label string, component schemaComponent, dst map[QName]schemaComponent) error {
	if _, exists := dst[q]; exists {
		return withSchemaCompileLocation(child, addSchemaComponent(dst, q, component, label))
	}
	if c.typeQNameKnown(q) {
		return withSchemaCompileLocation(child, schemaTypeNameConflictError(label))
	}
	return withSchemaCompileLocation(child, addSchemaComponent(dst, q, component, label))
}

func (c *compiler) indexModelGroup(child *schemaNode, q QName, label string, component schemaComponent) error {
	model, err := topLevelGroupModel(child)
	if err != nil {
		return err
	}
	if err := validateModelGroupSyntax(model, c.limits); err != nil {
		return err
	}
	return withSchemaCompileLocation(child, addSchemaComponent(c.groupComponents, q, component, label))
}

func (c *compiler) indexNotation(ctx *schemaSetDocument, n *schemaNode, name string) error {
	q, err := c.rt.internQName(ctx.effectiveTargetNS, name)
	if err != nil {
		return err
	}
	return withSchemaCompileLocation(n, c.addNotation(q, c.rt.formatName(q)))
}

// validateModelGroupSyntax checks the occurrence attributes of the model
// groups nested in n. Which particles n admits was decided as each started.
func validateModelGroupSyntax(n *schemaNode, limits Limits) error {
	for child := range n.xsdChildren() {
		switch child.local {
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
			if err := validateNestedModelGroupOccurrence(child, limits); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNestedModelGroupOccurrence(n *schemaNode, limits Limits) error {
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
