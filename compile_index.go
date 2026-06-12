package xsd

import (
	"cmp"
	"maps"
	"slices"
)

func (c *compiler) index() error {
	for _, doc := range c.docs {
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
	if target, ok := root.attr(xsdAttrTargetNamespace); ok && target == "" {
		return nil, schemaCompileAt(root, ErrSchemaInvalidAttribute, "schema targetNamespace cannot be empty")
	}
	blockDefault, err := parseDerivationSet(root.attrDefault(xsdAttrBlockDefault, ""), "schema blockDefault", derivationBlockDefaultMask)
	if err != nil {
		return nil, withSchemaCompileLocation(root, err)
	}
	finalDefault, err := parseDerivationSet(root.attrDefault(xsdAttrFinalDefault, ""), "schema finalDefault", derivationFinalDefaultMask)
	if err != nil {
		return nil, withSchemaCompileLocation(root, err)
	}
	elementQualified, err := schemaFormDefaultAttr(root, xsdAttrElementFormDefault)
	if err != nil {
		return nil, err
	}
	attrQualified, err := schemaFormDefaultAttr(root, xsdAttrAttributeFormDefault)
	if err != nil {
		return nil, err
	}
	ctx := &schemaContext{
		doc:              doc,
		targetNS:         root.attrDefault(xsdAttrTargetNamespace, ""),
		elementQualified: elementQualified,
		attrQualified:    attrQualified,
		blockDefault:     blockDefault,
		finalDefault:     finalDefault,
		imports:          c.imports[doc.key],
	}
	if ctx.targetNS == "" && c.adoptTarget[doc.key] != "" {
		ctx.targetNS = c.adoptTarget[doc.key]
	}
	return ctx, nil
}

func (c *compiler) indexTopLevelSchemaChild(child *rawNode, ctx *schemaContext) error {
	if err := c.validateTopLevelSchemaChild(child, ctx); err != nil {
		return err
	}
	name, ok := child.attr(xsdAttrName)
	if !ok {
		return nil
	}
	q, err := c.rt.Names.InternQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	label := c.rt.Names.Format(q)
	component := rawComponent{child, ctx}
	switch child.Name.Local {
	case xsdElemSimpleType:
		if _, exists := c.complexRaw[q]; exists {
			return schemaCompileAt(child, ErrSchemaDuplicate, "duplicate type "+label)
		}
		return addRaw(c.simpleRaw, q, component, label)
	case xsdElemComplexType:
		if _, exists := c.simpleRaw[q]; exists {
			return schemaCompileAt(child, ErrSchemaDuplicate, "duplicate type "+label)
		}
		return addRaw(c.complexRaw, q, component, label)
	case xsdElemElement:
		return addRaw(c.elementRaw, q, component, label)
	case xsdElemAttribute:
		// Builtin xml:* and xlink:* declarations take precedence over schema
		// redeclarations so that importing the W3C xml.xsd or xlink.xsd does
		// not raise duplicate-component errors. Only builtin names can collide
		// here; duplicates between schema documents still fail in addRaw.
		if _, exists := c.rt.GlobalAttributes[q]; exists {
			return nil
		}
		return addRaw(c.attributeRaw, q, component, label)
	case xsdElemGroup:
		if err := validateTopLevelGroupChildren(child, c.limits); err != nil {
			return err
		}
		return addRaw(c.groupRaw, q, component, label)
	case xsdElemAttributeGroup:
		return addRaw(c.attrGroupRaw, q, component, label)
	default:
		return nil
	}
}

func (c *compiler) validateTopLevelSchemaChild(child *rawNode, ctx *schemaContext) error {
	if !isTopLevelSchemaChild(child.Name.Local) {
		return schemaCompileAt(child, ErrSchemaContentModel, "invalid top-level schema child "+child.Name.Local)
	}
	switch child.Name.Local {
	case xsdElemAttributeGroup:
		return rejectTopLevelAttrs(child, xsdElemAttributeGroup, xsdAttrRef)
	case xsdElemGroup:
		return validateTopLevelGroupAttrs(child)
	case xsdElemUnique, xsdElemKey, xsdElemKeyref, xsdElemSelector, xsdElemField:
		return schemaCompileAt(child, ErrSchemaContentModel, "identity constraint must be inside element")
	case xsdElemNotation:
		return c.indexNotation(child, ctx)
	case xsdElemSimpleType, xsdElemComplexType:
		return requireTopLevelName(child)
	case xsdElemAttribute:
		return validateTopLevelAttributeAttrs(child)
	case xsdElemElement:
		return validateTopLevelElementAttrs(child)
	default:
		return nil
	}
}

func rejectTopLevelAttrs(n *rawNode, label string, attrs ...string) error {
	for _, attr := range attrs {
		if _, ok := n.attr(attr); ok {
			return schemaCompileAt(n, ErrSchemaInvalidAttribute, "top-level "+label+" cannot have "+attr)
		}
	}
	return nil
}

func requireTopLevelName(n *rawNode) error {
	if _, ok := n.attr(xsdAttrName); !ok {
		return schemaCompileAt(n, ErrSchemaReference, "top-level "+n.Name.Local+" missing name")
	}
	return nil
}

func validateTopLevelGroupAttrs(n *rawNode) error {
	if err := rejectTopLevelAttrs(n, xsdElemGroup, xsdAttrRef, xsdAttrMinOccurs, xsdAttrMaxOccurs); err != nil {
		return err
	}
	return requireTopLevelName(n)
}

func validateTopLevelAttributeAttrs(n *rawNode) error {
	if err := rejectTopLevelAttrs(n, xsdElemAttribute, xsdAttrRef, xsdAttrForm, xsdAttrUse); err != nil {
		return err
	}
	return requireTopLevelName(n)
}

func validateTopLevelElementAttrs(n *rawNode) error {
	if err := rejectTopLevelAttrs(n, xsdElemElement, xsdAttrRef, xsdAttrForm, xsdAttrMinOccurs, xsdAttrMaxOccurs); err != nil {
		return err
	}
	return requireTopLevelName(n)
}

func (c *compiler) indexNotation(n *rawNode, ctx *schemaContext) error {
	if trimXMLWhitespace(n.Text) != "" {
		return schemaCompileAt(n, ErrSchemaContentModel, "notation can contain only annotation")
	}
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI || child.Name.Local != xsdElemAnnotation {
			return schemaCompileAt(child, ErrSchemaContentModel, "notation can contain only annotation")
		}
	}
	name, ok := n.attr(xsdAttrName)
	if !ok {
		return schemaCompileAt(n, ErrSchemaInvalidAttribute, "notation missing name")
	}
	if _, hasPublic := n.attr(xsdAttrPublic); !hasPublic {
		if _, hasSystem := n.attr(xsdAttrSystem); !hasSystem {
			return schemaCompileAt(n, ErrSchemaInvalidAttribute, "notation requires public or system")
		}
	}
	q, err := c.rt.Names.InternQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	if c.rt.Notations[q] {
		return schemaCompileAt(n, ErrSchemaDuplicate, "duplicate notation "+c.rt.Names.Format(q))
	}
	c.rt.Notations[q] = true
	return nil
}

func isNotationAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrName, xsdAttrPublic, xsdAttrSystem:
		return true
	default:
		return false
	}
}

func isTopLevelSchemaChild(local string) bool {
	switch local {
	case xsdElemAnnotation, xsdElemInclude, xsdElemImport, "redefine", xsdElemSimpleType, xsdElemComplexType, xsdElemGroup, xsdElemAttributeGroup, xsdElemElement, xsdElemAttribute, xsdElemNotation:
		return true
	default:
		return false
	}
}

func addRaw(m map[qName]rawComponent, q qName, c rawComponent, label string) error {
	if _, exists := m[q]; exists {
		return schemaCompileAt(c.node, ErrSchemaDuplicate, "duplicate schema component "+label)
	}
	m[q] = c
	return nil
}

func validateTopLevelGroupChildren(n *rawNode, limits compileLimits) error {
	var model *rawNode
	seenAnnotation := false
	seenNonAnnotation := false
	for child := range n.xsdChildren() {
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenAnnotation {
				return schemaCompileAt(child, ErrSchemaContentModel, "top-level group can contain at most one annotation")
			}
			if seenNonAnnotation {
				return schemaCompileAt(child, ErrSchemaContentModel, "top-level group annotation must be first")
			}
			seenAnnotation = true
		case xsdElemSequence, xsdElemChoice, xsdElemAll:
			if model != nil {
				return schemaCompileAt(child, ErrSchemaContentModel, "top-level group must contain exactly one model group")
			}
			model = child
			seenNonAnnotation = true
		case xsdElemGroup:
			return schemaCompileAt(child, ErrSchemaContentModel, "top-level group cannot contain group ref")
		default:
			return schemaCompileAt(child, ErrSchemaContentModel, "invalid top-level group child "+child.Name.Local)
		}
	}
	if model == nil {
		return schemaCompileAt(n, ErrSchemaContentModel, "top-level group must contain exactly one model group")
	}
	if _, ok := model.attr(xsdAttrMinOccurs); ok {
		return schemaCompileAt(model, ErrSchemaOccurrence, "top-level model group cannot have minOccurs")
	}
	if _, ok := model.attr(xsdAttrMaxOccurs); ok {
		return schemaCompileAt(model, ErrSchemaOccurrence, "top-level model group cannot have maxOccurs")
	}
	return validateModelGroupSyntax(model, limits)
}

func validateModelGroupSyntax(n *rawNode, limits compileLimits) error {
	seenNonAnnotation := false
	for child := range n.xsdChildren() {
		if child.Name.Local == xsdElemAnnotation {
			if seenNonAnnotation {
				return schemaCompileAt(child, ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
			continue
		}
		seenNonAnnotation = true
		if n.Name.Local == xsdElemAll && child.Name.Local != xsdElemElement {
			return schemaCompileAt(child, ErrSchemaContentModel, "xs:all can contain only element particles")
		}
		switch child.Name.Local {
		case xsdElemElement:
		case xsdElemSequence, xsdElemChoice:
			if err := validateModelOccurrence(child, limits); err != nil {
				return err
			}
		case xsdElemAll:
			return schemaCompileAt(child, ErrSchemaContentModel, "xs:all cannot be nested in model groups")
		case xsdElemGroup:
			if _, ok := child.attr(xsdAttrRef); !ok {
				return schemaCompileAt(child, ErrSchemaReference, "group use missing ref")
			}
			if len(child.xsContentChildren()) != 0 {
				return schemaCompileAt(child, ErrSchemaContentModel, "group use can contain only annotation")
			}
		case xsdElemAny:
			if err := validateAnyParticleSyntax(child); err != nil {
				return err
			}
		default:
			return schemaCompileAt(child, ErrSchemaContentModel, "invalid model group child "+child.Name.Local)
		}
	}
	return nil
}

func sortedQNames[T any](m map[qName]T, names nameTable) []qName {
	return slices.SortedFunc(maps.Keys(m), func(a, b qName) int {
		aNS := names.Namespace(a.Namespace)
		bNS := names.Namespace(b.Namespace)
		if aNS != bNS {
			return cmp.Compare(aNS, bNS)
		}
		return cmp.Compare(names.Local(a.Local), names.Local(b.Local))
	})
}
