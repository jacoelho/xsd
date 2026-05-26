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
	for _, child := range doc.root.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if err := c.indexTopLevelSchemaChild(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) schemaContext(doc *rawDoc) (*schemaContext, error) {
	root := doc.root
	if target, ok := root.attr(xsdAttrTargetNamespace); ok && target == "" {
		return nil, schemaCompile(ErrSchemaInvalidAttribute, "schema targetNamespace cannot be empty")
	}
	blockDefault, err := parseDerivationSet(root.attrDefault(xsdAttrBlockDefault, ""), "schema blockDefault", derivationBlockDefaultMask)
	if err != nil {
		return nil, err
	}
	finalDefault, err := parseDerivationSet(root.attrDefault(xsdAttrFinalDefault, ""), "schema finalDefault", derivationFinalDefaultMask)
	if err != nil {
		return nil, err
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
			return schemaCompile(ErrSchemaDuplicate, "duplicate type "+label)
		}
		return addRaw(c.simpleRaw, q, component, label)
	case xsdElemComplexType:
		if _, exists := c.simpleRaw[q]; exists {
			return schemaCompile(ErrSchemaDuplicate, "duplicate type "+label)
		}
		return addRaw(c.complexRaw, q, component, label)
	case xsdElemElement:
		return addRaw(c.elementRaw, q, component, label)
	case xsdElemAttribute:
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
		return schemaCompile(ErrSchemaContentModel, "invalid top-level schema child "+child.Name.Local)
	}
	switch child.Name.Local {
	case xsdElemAttributeGroup:
		return rejectTopLevelAttrs(child, xsdElemAttributeGroup, xsdAttrRef)
	case xsdElemGroup:
		return validateTopLevelGroupAttrs(child)
	case xsdElemUnique, xsdElemKey, xsdElemKeyref, xsdElemSelector, xsdElemField:
		return schemaCompile(ErrSchemaContentModel, "identity constraint must be inside element")
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
			return schemaCompile(ErrSchemaInvalidAttribute, "top-level "+label+" cannot have "+attr)
		}
	}
	return nil
}

func requireTopLevelName(n *rawNode) error {
	if _, ok := n.attr(xsdAttrName); !ok {
		return schemaCompile(ErrSchemaReference, "top-level "+n.Name.Local+" missing name")
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
		return schemaCompile(ErrSchemaContentModel, "notation can contain only annotation")
	}
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI || child.Name.Local != xsdElemAnnotation {
			return schemaCompile(ErrSchemaContentModel, "notation can contain only annotation")
		}
	}
	name, ok := n.attr(xsdAttrName)
	if !ok {
		return schemaCompile(ErrSchemaInvalidAttribute, "notation missing name")
	}
	if _, hasPublic := n.attr(xsdAttrPublic); !hasPublic {
		if _, hasSystem := n.attr(xsdAttrSystem); !hasSystem {
			return schemaCompile(ErrSchemaInvalidAttribute, "notation requires public or system")
		}
	}
	q, err := c.rt.Names.InternQName(ctx.targetNS, name)
	if err != nil {
		return err
	}
	key := c.rt.Names.Format(q)
	if c.rt.Notations[key] {
		return schemaCompile(ErrSchemaDuplicate, "duplicate notation "+key)
	}
	c.rt.Notations[key] = true
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
		return schemaCompile(ErrSchemaDuplicate, "duplicate schema component "+label)
	}
	m[q] = c
	return nil
}

// validateTopLevelGroupChildren owns top-level group validation sequencing.
func validateTopLevelGroupChildren(n *rawNode, limits compileLimits) error {
	var model *rawNode
	seenAnnotation := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenAnnotation {
				return schemaCompile(ErrSchemaContentModel, "top-level group can contain at most one annotation")
			}
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "top-level group annotation must be first")
			}
			seenAnnotation = true
		case xsdElemSequence, xsdElemChoice, xsdElemAll:
			if model != nil {
				return schemaCompile(ErrSchemaContentModel, "top-level group must contain exactly one model group")
			}
			model = child
			seenNonAnnotation = true
		case xsdElemGroup:
			return schemaCompile(ErrSchemaContentModel, "top-level group cannot contain group ref")
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid top-level group child "+child.Name.Local)
		}
	}
	if model == nil {
		return schemaCompile(ErrSchemaContentModel, "top-level group must contain exactly one model group")
	}
	if _, ok := model.attr(xsdAttrMinOccurs); ok {
		return schemaCompile(ErrSchemaOccurrence, "top-level model group cannot have minOccurs")
	}
	if _, ok := model.attr(xsdAttrMaxOccurs); ok {
		return schemaCompile(ErrSchemaOccurrence, "top-level model group cannot have maxOccurs")
	}
	if model.Name.Local == xsdElemAll {
		for _, child := range model.xsContentChildren() {
			if child.Name.Local != xsdElemElement {
				continue
			}
			occurs, err := parseOccurs(child, limits)
			if err != nil {
				return err
			}
			if occurs.Unbounded || occurs.Max > 1 {
				return schemaCompile(ErrSchemaOccurrence, "xs:all particles cannot repeat")
			}
		}
	}
	return validateModelGroupSyntax(model, limits)
}

func validateModelGroupSyntax(n *rawNode, limits compileLimits) error {
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if child.Name.Local == xsdElemAnnotation {
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
			continue
		}
		seenNonAnnotation = true
		if n.Name.Local == xsdElemAll && child.Name.Local != xsdElemElement {
			return schemaCompile(ErrSchemaContentModel, "xs:all can contain only element particles")
		}
		switch child.Name.Local {
		case xsdElemElement:
		case xsdElemSequence, xsdElemChoice:
			if err := validateModelOccurrence(child, limits); err != nil {
				return err
			}
		case xsdElemAll:
			return schemaCompile(ErrSchemaContentModel, "xs:all cannot be nested in model groups")
		case xsdElemGroup:
			if _, ok := child.attr(xsdAttrRef); !ok {
				return schemaCompile(ErrSchemaReference, "group use missing ref")
			}
			if len(child.xsContentChildren()) != 0 {
				return schemaCompile(ErrSchemaContentModel, "group use can contain only annotation")
			}
		case xsdElemAny:
			if err := validateAnyParticleSyntax(child); err != nil {
				return err
			}
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid model group child "+child.Name.Local)
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
