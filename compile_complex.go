package xsd

import (
	"fmt"
	"slices"
)

func (c *compiler) compileComplexByQName(q qName) (complexTypeID, error) {
	if id, ok := c.complexDone[q]; ok {
		return id, nil
	}
	if c.compilingComplex[q] {
		if raw, ok := c.complexRaw[q]; ok {
			return noComplexType, schemaCompileAt(raw.node, ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(q))
		}
		return noComplexType, schemaCompile(ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(q))
	}
	raw, ok := c.complexRaw[q]
	if !ok {
		return noComplexType, schemaCompile(ErrSchemaReference, "unknown complex type "+c.rt.Names.Format(q))
	}
	c.compilingComplex[q] = true
	defer delete(c.compilingComplex, q)
	id, err := nextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return noComplexType, err
	}
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, TextType: noSimpleType, Base: complexRef(c.rt.Builtin.AnyType)})
	c.complexDone[q] = id
	c.rt.GlobalTypes[q] = complexRef(id)
	ct, err := c.compileComplexType(raw.node, raw.ctx, q, false)
	if err != nil {
		return noComplexType, err
	}
	block, err := complexBlockMaskWithDefault(raw.node, raw.ctx.blockDefault)
	if err != nil {
		return noComplexType, err
	}
	final, err := derivationMaskWithDefaultChecked(raw.node, raw.ctx.finalDefault, complexTypeFinalDerivation)
	if err != nil {
		return noComplexType, err
	}
	ct.Name = q
	ct.Block = block
	ct.Final = final
	c.rt.ComplexTypes[id] = ct
	return id, nil
}

func (c *compiler) compileAnonymousComplex(n *rawNode, ctx *schemaContext) (complexTypeID, error) {
	if _, ok := n.attr(xsdAttrName); ok {
		return noComplexType, schemaCompileAt(n, ErrSchemaInvalidAttribute, "local complexType cannot have name")
	}
	q, err := c.rt.Names.InternQName("", fmt.Sprintf("$complex%d", len(c.rt.ComplexTypes)))
	if err != nil {
		return noComplexType, err
	}
	id, err := nextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return noComplexType, err
	}
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, TextType: noSimpleType, Base: complexRef(c.rt.Builtin.AnyType)})
	ct, err := c.compileComplexType(n, ctx, q, true)
	if err != nil {
		return noComplexType, err
	}
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, complexTypeFinalDerivation)
	if err != nil {
		return noComplexType, err
	}
	ct.Name = q
	ct.Final = final
	c.rt.ComplexTypes[id] = ct
	return id, nil
}

func schemaBoolAttr(n *rawNode, name string) (bool, error) {
	return schemaBoolAttrDefault(n, name, false)
}

func schemaBoolAttrDefault(n *rawNode, name string, def bool) (bool, error) {
	v, ok := n.attr(name)
	if !ok {
		return def, nil
	}
	b, valid := parseSchemaBool(v)
	if !valid {
		return false, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid boolean attribute "+name)
	}
	return b, nil
}

func schemaFormDefaultAttr(n *rawNode, name string) (bool, error) {
	v, ok := n.attr(name)
	if !ok {
		return false, nil
	}
	switch v {
	case xsdValueQualified:
		return true, nil
	case xsdValueUnqualified:
		return false, nil
	default:
		return false, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid "+name+" value "+v)
	}
}

func validateKnownAttributes(n *rawNode, label string, allowed func(string) bool) error {
	for _, attr := range n.Attr {
		if isNamespaceAttr(attr) || attr.Name.Space != "" {
			continue
		}
		if !allowed(attr.Name.Local) {
			return schemaCompileAt(n, ErrSchemaInvalidAttribute, label+" cannot have attribute "+attr.Name.Local)
		}
	}
	return nil
}

var complexTypeChildOrder = childOrder{
	annotationFirstMsg: "complexType annotation must be first",
	rules: []childRule{
		{
			match:    matchLocal(xsdElemSimpleContent, xsdElemComplexContent),
			level:    0,
			terminal: true,
			orderMsg: "complexType content model is out of order",
		},
		{
			match:    matchLocal(xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup),
			level:    1,
			maxOne:   true,
			orderMsg: "complexType model group is out of order",
			dupMsg:   "complexType model group is out of order",
		},
		{
			match:    matchLocal(xsdElemAttribute, xsdElemAttributeGroup),
			level:    2,
			orderMsg: "complexType attribute is out of order",
		},
		{
			match:  matchLocal(xsdElemAnyAttribute),
			level:  3,
			maxOne: true,
			dupMsg: "complexType can contain at most one anyAttribute",
		},
	},
	invalidMsg: func(local string) string { return "invalid complexType child " + local },
}

func validateComplexTypeContent(n *rawNode) error {
	return checkOrderedChildren(n, complexTypeChildOrder)
}

func (c *compiler) compileComplexType(n *rawNode, ctx *schemaContext, name qName, anonymous bool) (complexType, error) {
	if err := validateComplexTypeContent(n); err != nil {
		return complexType{}, err
	}
	mixed, err := schemaBoolAttr(n, xsdAttrMixed)
	if err != nil {
		return complexType{}, err
	}
	abstract, err := schemaBoolAttr(n, xsdAttrAbstract)
	if err != nil {
		return complexType{}, err
	}
	block, err := complexBlockMaskWithDefault(n, ctx.blockDefault)
	if err != nil {
		return complexType{}, err
	}
	ct := complexType{
		Name:        name,
		Content:     noContentModel,
		Attrs:       noAttributeUseSet,
		TextType:    noSimpleType,
		ContentKind: elementContentKind(mixed),
		Abstract:    abstract,
		Base:        complexRef(c.rt.Builtin.AnyType),
		Derivation:  derivationRestriction,
		Block:       block,
	}
	if cc := n.firstXS(xsdElemComplexContent); cc != nil {
		return c.compileComplexContent(cc, ctx, ct, anonymous)
	}
	if sc := n.firstXS(xsdElemSimpleContent); sc != nil {
		return c.compileSimpleContent(sc, ctx, ct, anonymous)
	}
	for _, child := range n.xsContentChildren() {
		switch child.Name.Local {
		case xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup:
			if occurrenceErr := validateModelOccurrence(child, c.limits); occurrenceErr != nil {
				return complexType{}, occurrenceErr
			}
			modelID, modelErr := c.compileModel(child, ctx)
			if modelErr != nil {
				return complexType{}, modelErr
			}
			ct.Content = modelID
		}
	}
	if ct.Content == noContentModel {
		ct.Content, err = c.addModel(contentModel{Kind: modelEmpty, Mixed: mixed})
		if err != nil {
			return complexType{}, err
		}
	}
	attrs, err := c.compileAttributeUses(n, ctx, nil, noWildcard, attributeMergeNormal)
	if err != nil {
		return complexType{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileComplexContent(n *rawNode, ctx *schemaContext, ct complexType, anonymous bool) (complexType, error) {
	if err := validateComplexContentChildren(n); err != nil {
		return complexType{}, err
	}
	mixed, err := schemaBoolAttrDefault(n, xsdAttrMixed, ct.mixed())
	if err != nil {
		return complexType{}, err
	}
	for _, child := range n.xsContentChildren() {
		if child.Name.Local != xsdElemExtension && child.Name.Local != xsdElemRestriction {
			continue
		}
		return c.compileComplexContentDerivation(child, ctx, ct, mixed, anonymous)
	}
	return complexType{}, schemaCompileAt(n, ErrSchemaContentModel, "complexContent missing extension or restriction")
}

func (c *compiler) compileComplexContentDerivation(child *rawNode, ctx *schemaContext, ct complexType, mixed, anonymous bool) (complexType, error) {
	baseID, base, err := c.complexContentBase(child, ctx, anonymous)
	if err != nil {
		return complexType{}, err
	}
	if mixed && !base.mixed() {
		return complexType{}, schemaCompileAt(child, ErrSchemaContentModel, "complexContent mixed derivation requires mixed base")
	}
	if err := validateComplexContentDerivationChildren(child); err != nil {
		return complexType{}, err
	}
	ct.Base = complexRef(baseID)
	if child.Name.Local == xsdElemExtension {
		return c.compileComplexContentExtension(child, ctx, ct, baseID, base, mixed)
	}
	return c.compileComplexContentRestriction(child, ctx, ct, base, mixed)
}

func (c *compiler) complexContentBase(child *rawNode, ctx *schemaContext, anonymous bool) (complexTypeID, complexType, error) {
	baseLex, ok := child.attr(xsdAttrBase)
	if !ok {
		return noComplexType, complexType{}, schemaCompileAt(child, ErrSchemaReference, "complexContent "+child.Name.Local+" missing base")
	}
	baseQName, err := c.resolveQNameChecked(child, ctx, baseLex)
	if err != nil {
		return noComplexType, complexType{}, err
	}
	if c.compilingComplex[baseQName] && !anonymous {
		return noComplexType, complexType{}, schemaCompileAt(child, ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(baseQName))
	}
	baseID, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return noComplexType, complexType{}, withSchemaCompileLocation(child, err)
	}
	return baseID, c.rt.ComplexTypes[baseID], nil
}

func (c *compiler) compileComplexContentExtension(child *rawNode, ctx *schemaContext, ct complexType, baseID complexTypeID, base complexType, mixed bool) (complexType, error) {
	if base.Final&blockExtension != 0 {
		return complexType{}, schemaCompileAt(child, ErrSchemaReference, "base complex type final blocks extension")
	}
	if base.simpleContent() {
		return c.compileSimpleValueComplexExtension(child, ctx, ct, base, mixed)
	}
	ct.Derivation = derivationExtension
	ct.Content = base.Content
	ct.Attrs = base.Attrs
	if modelNode := firstModelChild(child); modelNode != nil {
		content, err := c.compileComplexExtensionModel(modelNode, ctx, baseID, base, mixed)
		if err != nil {
			return complexType{}, err
		}
		ct.Content = content
	}
	baseUses, baseWildcard := c.attrUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, attributeMergeNormal)
	if err != nil {
		return complexType{}, err
	}
	ct.Attrs = attrs
	ct.ContentKind = elementContentKind(base.mixed() || mixed)
	return ct, nil
}

func (c *compiler) compileSimpleValueComplexExtension(child *rawNode, ctx *schemaContext, ct, base complexType, mixed bool) (complexType, error) {
	if firstModelChild(child) != nil {
		return complexType{}, schemaCompileAt(child, ErrSchemaContentModel, "complexContent extension cannot add particles to simple content")
	}
	baseUses, baseWildcard := c.attrUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, attributeMergeNormal)
	if err != nil {
		return complexType{}, err
	}
	ct.Derivation = derivationExtension
	ct.Content, err = c.addModel(contentModel{Kind: modelEmpty})
	if err != nil {
		return complexType{}, err
	}
	ct.Attrs = attrs
	ct.TextType = base.TextType
	ct.ContentKind = simpleContentKind(mixed)
	return ct, nil
}

func (c *compiler) compileComplexExtensionModel(modelNode *rawNode, ctx *schemaContext, baseID complexTypeID, base complexType, mixed bool) (contentModelID, error) {
	if baseID != c.rt.Builtin.AnyType && base.mixed() && !mixed {
		return noContentModel, schemaCompileAt(modelNode, ErrSchemaContentModel, "complexContent extension cannot drop mixed base content")
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return noContentModel, err
	}
	if modelNode.Name.Local == xsdElemAll && !c.modelHasNoParticles(base.Content) {
		return noContentModel, schemaCompileAt(modelNode, ErrSchemaContentModel, "complexContent extension cannot use xs:all")
	}
	if base.Content != noContentModel {
		baseModel := c.rt.Models[base.Content]
		if baseModel.Kind == modelAll && len(baseModel.Particles) != 0 {
			return noContentModel, schemaCompileAt(modelNode, ErrSchemaContentModel, "complexContent extension cannot add particles to xs:all base")
		}
	}
	ext, err := c.compileModel(modelNode, ctx)
	if err != nil {
		return noContentModel, err
	}
	return c.extendSequence(base.Content, ext)
}

func (c *compiler) compileComplexContentRestriction(child *rawNode, ctx *schemaContext, ct, base complexType, mixed bool) (complexType, error) {
	if base.Final&blockRestriction != 0 {
		return complexType{}, schemaCompileAt(child, ErrSchemaReference, "base complex type final blocks restriction")
	}
	if base.simpleContent() {
		return complexType{}, schemaCompileAt(child, ErrSchemaContentModel, "complexContent restriction base cannot have simple content")
	}
	ct.Derivation = derivationRestriction
	content, err := c.compileComplexRestrictionModel(child, ctx, ct)
	if err != nil {
		return complexType{}, err
	}
	ct.Content = content
	baseUses, baseWildcard := c.attrUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, attributeMergeRestriction)
	if err != nil {
		return complexType{}, err
	}
	ct.Attrs = attrs
	ct.ContentKind = elementContentKind(mixed)
	return ct, nil
}

func (c *compiler) compileComplexRestrictionModel(child *rawNode, ctx *schemaContext, ct complexType) (contentModelID, error) {
	modelNode := firstModelChild(child)
	if modelNode == nil {
		return c.addModel(contentModel{Kind: modelEmpty, Mixed: ct.mixed()})
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return noContentModel, err
	}
	return c.compileModel(modelNode, ctx)
}

var complexContentChildOrder = derivationContainerOrder("complexContent")
var simpleContentChildOrder = derivationContainerOrder("simpleContent")

func validateComplexContentChildren(n *rawNode) error {
	return checkOrderedChildren(n, complexContentChildOrder)
}

func validateSimpleContentChildren(n *rawNode) error {
	return checkOrderedChildren(n, simpleContentChildOrder)
}

func derivationContainerOrder(label string) childOrder {
	return childOrder{
		annotationFirstMsg: label + " annotation must be first",
		rules: []childRule{
			{
				match:  matchLocal(xsdElemExtension, xsdElemRestriction),
				maxOne: true,
				dupMsg: label + " can contain one derivation",
			},
		},
		invalidMsg: func(local string) string { return "invalid " + label + " child " + local },
	}
}

var simpleContentRestrictionChildOrder = simpleContentDerivationOrder(xsdElemRestriction)
var simpleContentExtensionChildOrder = simpleContentDerivationOrder(xsdElemExtension)

func validateSimpleContentDerivationChildren(n *rawNode) error {
	if n.Name.Local == xsdElemRestriction {
		return checkOrderedChildren(n, simpleContentRestrictionChildOrder)
	}
	return checkOrderedChildren(n, simpleContentExtensionChildOrder)
}

func simpleContentDerivationOrder(derivation string) childOrder {
	rules := []childRule{
		{
			match:    matchLocal(xsdElemSimpleType),
			level:    0,
			maxOne:   true,
			orderMsg: "simpleContent simpleType is out of order",
			dupMsg:   "simpleContent simpleType is out of order",
		},
		{
			match:    matchLocal(xsdElemAttribute, xsdElemAttributeGroup),
			level:    1,
			orderMsg: "simpleContent attribute is out of order",
		},
		{
			match:  matchLocal(xsdElemAnyAttribute),
			level:  2,
			maxOne: true,
			dupMsg: "simpleContent can contain at most one anyAttribute",
		},
	}
	if derivation == xsdElemRestriction {
		rules = append(rules, childRule{
			match:    isFacetNode,
			level:    0,
			orderMsg: "simpleContent facet is out of order",
		})
	} else {
		rules[0].forbiddenMsg = "simpleContent extension cannot contain simpleType"
	}
	return childOrder{
		annotationFirstMsg: derivation + " annotation must be first",
		rules:              rules,
		invalidMsg: func(local string) string {
			return "invalid simpleContent " + derivation + " child " + local
		},
	}
}

func isFacetNode(local string) bool {
	switch local {
	case xsdFacetLength, xsdFacetMinLength, xsdFacetMaxLength, xsdFacetTotalDigits, xsdFacetFractionDigits,
		xsdFacetMinInclusive, xsdFacetMaxInclusive, xsdFacetMinExclusive, xsdFacetMaxExclusive,
		xsdFacetEnumeration, xsdFacetPattern, xsdFacetWhiteSpace:
		return true
	default:
		return false
	}
}

func (c *compiler) compileSimpleContent(n *rawNode, ctx *schemaContext, ct complexType, anonymous bool) (complexType, error) {
	if err := validateSimpleContentChildren(n); err != nil {
		return complexType{}, err
	}
	child := simpleContentDerivationChild(n)
	if child == nil {
		return complexType{}, schemaCompileAt(n, ErrSchemaContentModel, "simpleContent missing extension or restriction")
	}
	if err := validateSimpleContentDerivationChildren(child); err != nil {
		return complexType{}, err
	}
	baseLex, ok := child.attr(xsdAttrBase)
	if !ok {
		return complexType{}, schemaCompileAt(child, ErrSchemaReference, "simpleContent "+child.Name.Local+" missing base")
	}
	baseQName, err := c.resolveQNameChecked(child, ctx, baseLex)
	if err != nil {
		return complexType{}, err
	}
	isRestriction := child.Name.Local == xsdElemRestriction
	var textType simpleTypeID
	if c.simpleTypeQNameKnown(baseQName) {
		ct, textType, err = c.compileSimpleContentSimpleBase(child, baseQName, ct)
	} else {
		ct, textType, err = c.compileSimpleContentComplexBase(child, baseQName, ct, anonymous)
	}
	if err != nil {
		return complexType{}, err
	}
	mergeMode := attributeMergeNormal
	derivation := derivationExtension
	if isRestriction {
		textType, err = c.compileSimpleContentRestrictionType(child, ctx, textType)
		if err != nil {
			return complexType{}, err
		}
		mergeMode = attributeMergeRestriction
		derivation = derivationRestriction
	}
	inheritedUses, inheritedWildcard := c.attrUsesAndWildcard(ct.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, inheritedUses, inheritedWildcard, mergeMode)
	if err != nil {
		return complexType{}, err
	}
	ct.Attrs = attrs
	ct.Content, err = c.addModel(contentModel{Kind: modelEmpty})
	if err != nil {
		return complexType{}, err
	}
	ct.TextType = textType
	ct.ContentKind = simpleContentKind(ct.mixed())
	ct.Derivation = derivation
	return ct, nil
}

func simpleContentDerivationChild(n *rawNode) *rawNode {
	for _, child := range n.xsContentChildren() {
		if child.Name.Local == xsdElemExtension || child.Name.Local == xsdElemRestriction {
			return child
		}
	}
	return nil
}

func (c *compiler) compileSimpleContentSimpleBase(child *rawNode, baseQName qName, ct complexType) (complexType, simpleTypeID, error) {
	simpleID, err := c.compileSimpleByQName(baseQName)
	if err != nil {
		return complexType{}, noSimpleType, withSchemaCompileLocation(child, err)
	}
	if child.Name.Local == xsdElemRestriction {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaContentModel, "simpleContent restriction base must be complex type")
	}
	if c.rt.SimpleTypes[simpleID].Final&blockExtension != 0 {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaReference, "base simple type final blocks extension")
	}
	ct.Base = simpleRef(simpleID)
	return ct, simpleID, nil
}

func (c *compiler) compileSimpleContentComplexBase(child *rawNode, baseQName qName, ct complexType, anonymous bool) (complexType, simpleTypeID, error) {
	if c.compilingComplex[baseQName] && !anonymous {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(baseQName))
	}
	if !c.complexTypeQNameKnown(baseQName) {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaReference, "simpleContent base must be simple or simple-content complex type")
	}
	baseComplex, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return complexType{}, noSimpleType, withSchemaCompileLocation(child, err)
	}
	base := c.rt.ComplexTypes[baseComplex]
	if !base.simpleContent() {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaContentModel, "simpleContent base must have simple content")
	}
	if child.Name.Local == xsdElemExtension && base.Final&blockExtension != 0 {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaReference, "base complex type final blocks extension")
	}
	if child.Name.Local == xsdElemRestriction && base.Final&blockRestriction != 0 {
		return complexType{}, noSimpleType, schemaCompileAt(child, ErrSchemaReference, "base complex type final blocks restriction")
	}
	ct.Base = complexRef(baseComplex)
	ct.Attrs = base.Attrs
	return ct, base.TextType, nil
}

func (c *compiler) compileSimpleContentRestrictionType(child *rawNode, ctx *schemaContext, baseTextType simpleTypeID) (simpleTypeID, error) {
	textType := baseTextType
	facetChildren := facetChildren(child)
	if stNode := child.firstXS(xsdElemSimpleType); stNode != nil {
		simpleID, err := c.compileAnonymousSimple(stNode, ctx)
		if err != nil {
			return noSimpleType, err
		}
		textType = simpleID
	}
	if len(facetChildren) != 0 {
		simpleID, err := c.compileSimpleContentFacetRestriction(facetChildren, textType)
		if err != nil {
			return noSimpleType, err
		}
		textType = simpleID
	}
	if !c.typeDerivesFrom(simpleRef(textType), simpleRef(baseTextType)) {
		return noSimpleType, schemaCompileAt(child, ErrSchemaContentModel, "simpleContent restriction type is not derived from base")
	}
	return textType, nil
}

func (c *compiler) compileSimpleContentFacetRestriction(facetChildren []*rawNode, baseID simpleTypeID) (simpleTypeID, error) {
	if baseID == c.rt.Builtin.AnySimpleType {
		return noSimpleType, schemaCompileAt(facetChildren[0], ErrSchemaReference, "simple type cannot restrict xs:anySimpleType")
	}
	base := c.rt.SimpleTypes[baseID]
	if base.Final&blockRestriction != 0 {
		return noSimpleType, schemaCompileAt(facetChildren[0], ErrSchemaReference, "base simple type final blocks restriction")
	}
	q, err := c.rt.Names.InternQName("", fmt.Sprintf("$simple%d", len(c.rt.SimpleTypes)))
	if err != nil {
		return noSimpleType, err
	}
	st := base
	st.Name = q
	st.Base = baseID
	st.Final = 0
	st.Facets = cloneFacetSet(base.Facets)
	st.Union = slices.Clone(base.Union)
	if err = c.compileFacetList(facetChildren, &st, baseID, baseID); err != nil {
		return noSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	id, err := nextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return noSimpleType, err
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, st)
	return id, nil
}

func facetChildren(n *rawNode) []*rawNode {
	var out []*rawNode
	for _, child := range n.Children {
		if child.Name.Space == xsdNamespaceURI && isFacetNode(child.Name.Local) {
			out = append(out, child)
		}
	}
	return out
}

func firstModelChild(n *rawNode) *rawNode {
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup:
			return child
		}
	}
	return nil
}

var complexContentRestrictionChildOrder = complexContentDerivationOrder(xsdElemRestriction)
var complexContentExtensionChildOrder = complexContentDerivationOrder(xsdElemExtension)

func validateComplexContentDerivationChildren(n *rawNode) error {
	if n.Name.Local == xsdElemRestriction {
		return checkOrderedChildren(n, complexContentRestrictionChildOrder)
	}
	return checkOrderedChildren(n, complexContentExtensionChildOrder)
}

func complexContentDerivationOrder(derivation string) childOrder {
	return childOrder{
		annotationFirstMsg: derivation + " annotation must be first",
		rules: []childRule{
			{
				match:    matchLocal(xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup),
				level:    0,
				maxOne:   true,
				orderMsg: derivation + " model group is out of order",
				dupMsg:   derivation + " model group is out of order",
			},
			{
				match:    matchLocal(xsdElemAttribute, xsdElemAttributeGroup),
				level:    1,
				orderMsg: derivation + " attribute is out of order",
			},
			{
				match:  matchLocal(xsdElemAnyAttribute),
				level:  2,
				maxOne: true,
				dupMsg: derivation + " can contain at most one anyAttribute",
			},
		},
		invalidMsg: func(local string) string { return "invalid complexContent child " + local },
	}
}
