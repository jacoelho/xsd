package xsd

import (
	"maps"
	"slices"
	"strings"
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
	if target, ok := root.attr("targetNamespace"); ok && target == "" {
		return nil, schemaCompile(ErrSchemaInvalidAttribute, "schema targetNamespace cannot be empty")
	}
	blockDefault, err := parseDerivationMaskChecked(root.attrDefault("blockDefault", ""), true, "schema blockDefault")
	if err != nil {
		return nil, err
	}
	elementQualified, err := schemaFormDefaultAttr(root, "elementFormDefault")
	if err != nil {
		return nil, err
	}
	attrQualified, err := schemaFormDefaultAttr(root, "attributeFormDefault")
	if err != nil {
		return nil, err
	}
	ctx := &schemaContext{
		doc:              doc,
		targetNS:         root.attrDefault("targetNamespace", ""),
		elementQualified: elementQualified,
		attrQualified:    attrQualified,
		blockDefault:     blockDefault,
		finalDefault:     parseDerivationMask(root.attrDefault("finalDefault", "")),
		imports:          c.imports[doc.name],
	}
	if ctx.targetNS == "" && c.adoptTarget[doc.name] != "" {
		ctx.targetNS = c.adoptTarget[doc.name]
	}
	return ctx, nil
}

func (c *compiler) indexTopLevelSchemaChild(child *rawNode, ctx *schemaContext) error {
	if err := c.validateTopLevelSchemaChild(child, ctx); err != nil {
		return err
	}
	name, ok := child.attr("name")
	if !ok {
		return nil
	}
	q := c.rt.Names.InternQName(ctx.targetNS, name)
	label := c.rt.Names.Format(q)
	component := rawComponent{child, ctx}
	switch child.Name.Local {
	case "simpleType":
		if _, exists := c.complexRaw[q]; exists {
			return schemaCompile(ErrSchemaDuplicate, "duplicate type "+label)
		}
		return addRaw(c.simpleRaw, q, component, label)
	case "complexType":
		if _, exists := c.simpleRaw[q]; exists {
			return schemaCompile(ErrSchemaDuplicate, "duplicate type "+label)
		}
		return addRaw(c.complexRaw, q, component, label)
	case "element":
		return addRaw(c.elementRaw, q, component, label)
	case "attribute":
		if _, exists := c.rt.GlobalAttributes[q]; exists {
			return nil
		}
		return addRaw(c.attributeRaw, q, component, label)
	case "group":
		if err := validateTopLevelGroupChildren(child, c.limits); err != nil {
			return err
		}
		return addRaw(c.groupRaw, q, component, label)
	case "attributeGroup":
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
	case "attributeGroup":
		return rejectTopLevelAttrs(child, "attributeGroup", "ref")
	case "group":
		return validateTopLevelGroupAttrs(child)
	case "unique", "key", "keyref", "selector", "field":
		return schemaCompile(ErrSchemaContentModel, "identity constraint must be inside element")
	case "notation":
		return c.indexNotation(child, ctx)
	case "simpleType", "complexType":
		return requireTopLevelName(child)
	case "attribute":
		return validateTopLevelAttributeAttrs(child)
	case "element":
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
	if _, ok := n.attr("name"); !ok {
		return schemaCompile(ErrSchemaReference, "top-level "+n.Name.Local+" missing name")
	}
	return nil
}

func validateTopLevelGroupAttrs(n *rawNode) error {
	if err := rejectTopLevelAttrs(n, "group", "ref", "minOccurs", "maxOccurs"); err != nil {
		return err
	}
	return requireTopLevelName(n)
}

func validateTopLevelAttributeAttrs(n *rawNode) error {
	if err := rejectTopLevelAttrs(n, "attribute", "ref", "form", "use"); err != nil {
		return err
	}
	return requireTopLevelName(n)
}

func validateTopLevelElementAttrs(n *rawNode) error {
	if err := rejectTopLevelAttrs(n, "element", "ref", "form", "minOccurs", "maxOccurs"); err != nil {
		return err
	}
	return requireTopLevelName(n)
}

func (c *compiler) indexNotation(n *rawNode, ctx *schemaContext) error {
	if err := validateKnownAttributes(n, "notation", map[string]bool{
		"id":     true,
		"name":   true,
		"public": true,
		"system": true,
	}); err != nil {
		return err
	}
	if strings.TrimSpace(n.Text) != "" {
		return schemaCompile(ErrSchemaContentModel, "notation can contain only annotation")
	}
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI || child.Name.Local != "annotation" {
			return schemaCompile(ErrSchemaContentModel, "notation can contain only annotation")
		}
	}
	name, ok := n.attr("name")
	if !ok {
		return schemaCompile(ErrSchemaInvalidAttribute, "notation missing name")
	}
	if _, hasPublic := n.attr("public"); !hasPublic {
		if _, hasSystem := n.attr("system"); !hasSystem {
			return schemaCompile(ErrSchemaInvalidAttribute, "notation requires public or system")
		}
	}
	q := c.rt.Names.InternQName(ctx.targetNS, name)
	key := c.rt.Names.Format(q)
	if c.rt.Notations[key] {
		return schemaCompile(ErrSchemaDuplicate, "duplicate notation "+key)
	}
	c.rt.Notations[key] = true
	return nil
}

func isTopLevelSchemaChild(local string) bool {
	switch local {
	case "annotation", "include", "import", "redefine", "simpleType", "complexType", "group", "attributeGroup", "element", "attribute", "notation":
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

func validateTopLevelGroupChildren(n *rawNode, limits compileLimits) error {
	var model *rawNode
	seenAnnotation := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case "annotation":
			if seenAnnotation {
				return schemaCompile(ErrSchemaContentModel, "top-level group can contain at most one annotation")
			}
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "top-level group annotation must be first")
			}
			seenAnnotation = true
		case "sequence", "choice", "all":
			if model != nil {
				return schemaCompile(ErrSchemaContentModel, "top-level group must contain exactly one model group")
			}
			model = child
			seenNonAnnotation = true
		case "group":
			return schemaCompile(ErrSchemaContentModel, "top-level group cannot contain group ref")
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid top-level group child "+child.Name.Local)
		}
	}
	if model == nil {
		return schemaCompile(ErrSchemaContentModel, "top-level group must contain exactly one model group")
	}
	return validateTopLevelGroupModel(model, limits)
}

func validateTopLevelGroupModel(model *rawNode, limits compileLimits) error {
	if model == nil {
		return nil
	}
	if _, ok := model.attr("minOccurs"); ok {
		return schemaCompile(ErrSchemaOccurrence, "top-level model group cannot have minOccurs")
	}
	if _, ok := model.attr("maxOccurs"); ok {
		return schemaCompile(ErrSchemaOccurrence, "top-level model group cannot have maxOccurs")
	}
	if model.Name.Local == "all" {
		for _, child := range model.xsContentChildren() {
			if child.Name.Local != "element" {
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
		if child.Name.Local == "annotation" {
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
			continue
		}
		seenNonAnnotation = true
		if n.Name.Local == "all" && child.Name.Local != "element" {
			return schemaCompile(ErrSchemaContentModel, "xs:all can contain only element particles")
		}
		switch child.Name.Local {
		case "element":
		case "sequence", "choice":
			if err := validateModelOccurrence(child, limits); err != nil {
				return err
			}
			if err := validateModelGroupSyntax(child, limits); err != nil {
				return err
			}
		case "all":
			return schemaCompile(ErrSchemaContentModel, "xs:all cannot be nested in model groups")
		case "group":
			if _, ok := child.attr("ref"); !ok {
				return schemaCompile(ErrSchemaReference, "group use missing ref")
			}
			if len(child.xsContentChildren()) != 0 {
				return schemaCompile(ErrSchemaContentModel, "group use can contain only annotation")
			}
		case "any":
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
			return strings.Compare(aNS, bNS)
		}
		return strings.Compare(names.Local(a.Local), names.Local(b.Local))
	})
}
