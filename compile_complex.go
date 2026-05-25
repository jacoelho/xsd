package xsd

import (
	"fmt"
	"slices"
	"strings"
)

func (c *compiler) compileComplexByQName(q qName) (complexTypeID, error) {
	if id, ok := c.complexDone[q]; ok {
		return id, nil
	}
	if c.compilingComplex[q] {
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
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, Base: typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}})
	c.complexDone[q] = id
	c.rt.GlobalTypes[q] = typeID{Kind: typeComplex, ID: uint32(id)}
	ct, err := c.compileComplexType(raw.node, raw.ctx, q)
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
		return noComplexType, schemaCompile(ErrSchemaInvalidAttribute, "local complexType cannot have name")
	}
	q, err := c.rt.Names.InternQName("", fmt.Sprintf("$complex%d", len(c.rt.ComplexTypes)))
	if err != nil {
		return noComplexType, err
	}
	id, err := nextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return noComplexType, err
	}
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, Base: typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}})
	ct, err := c.compileComplexType(n, ctx, q)
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

func (c *compiler) isAnonymousComplexName(q qName) bool {
	return c.rt.Names.Namespace(q.Namespace) == "" && strings.HasPrefix(c.rt.Names.Local(q.Local), "$complex")
}

func schemaBoolAttr(n *rawNode, name string) (bool, error) {
	v, ok := n.attr(name)
	if !ok {
		return false, nil
	}
	b, valid := parseSchemaBool(v)
	if !valid {
		return false, schemaCompile(ErrSchemaInvalidAttribute, "invalid boolean attribute "+name)
	}
	return b, nil
}

func schemaBoolAttrDefault(n *rawNode, name string, def bool) (bool, error) {
	v, ok := n.attr(name)
	if !ok {
		return def, nil
	}
	b, valid := parseSchemaBool(v)
	if !valid {
		return false, schemaCompile(ErrSchemaInvalidAttribute, "invalid boolean attribute "+name)
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
		return false, schemaCompile(ErrSchemaInvalidAttribute, "invalid "+name+" value "+v)
	}
}

func validateKnownAttributes(n *rawNode, label string, allowed func(string) bool) error {
	for _, attr := range n.Attr {
		if isNamespaceAttr(attr) || attr.Name.Space != "" {
			continue
		}
		if !allowed(attr.Name.Local) {
			return schemaCompile(ErrSchemaInvalidAttribute, label+" cannot have attribute "+attr.Name.Local)
		}
	}
	return nil
}

func validateComplexTypeContent(n *rawNode) error {
	sawModel := false
	sawAttr := false
	sawAnyAttr := false
	terminal := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if terminal {
			return schemaCompile(ErrSchemaContentModel, "invalid complexType child "+child.Name.Local)
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if sawModel || sawAttr || sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType annotation must be first")
			}
		case xsdElemSimpleContent, xsdElemComplexContent:
			if sawModel || sawAttr || sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType content model is out of order")
			}
			terminal = true
		case xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup:
			if sawModel || sawAttr || sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType model group is out of order")
			}
			sawModel = true
		case xsdElemAttribute, xsdElemAttributeGroup:
			if sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType attribute is out of order")
			}
			sawAttr = true
		case xsdElemAnyAttribute:
			if sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType can contain at most one anyAttribute")
			}
			sawAnyAttr = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid complexType child "+child.Name.Local)
		}
	}
	return nil
}

func (c *compiler) compileComplexType(n *rawNode, ctx *schemaContext, name qName) (complexType, error) {
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
		Name:       name,
		Content:    noContentModel,
		Attrs:      noAttributeUseSet,
		TextType:   noSimpleType,
		Mixed:      mixed,
		Abstract:   abstract,
		Base:       typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)},
		Derivation: derivationRestriction,
		Block:      block,
	}
	if cc := n.firstXS(xsdElemComplexContent); cc != nil {
		return c.compileComplexContent(cc, ctx, ct)
	}
	if sc := n.firstXS(xsdElemSimpleContent); sc != nil {
		return c.compileSimpleContent(sc, ctx, ct)
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

func (c *compiler) compileComplexContent(n *rawNode, ctx *schemaContext, ct complexType) (complexType, error) {
	if err := validateComplexContentChildren(n); err != nil {
		return complexType{}, err
	}
	mixed, err := schemaBoolAttrDefault(n, xsdAttrMixed, ct.Mixed)
	if err != nil {
		return complexType{}, err
	}
	for _, child := range n.xsContentChildren() {
		if child.Name.Local != xsdElemExtension && child.Name.Local != xsdElemRestriction {
			continue
		}
		return c.compileComplexContentDerivation(child, ctx, ct, mixed)
	}
	return complexType{}, schemaCompile(ErrSchemaContentModel, "complexContent missing extension or restriction")
}

func (c *compiler) compileComplexContentDerivation(child *rawNode, ctx *schemaContext, ct complexType, mixed bool) (complexType, error) {
	baseID, base, err := c.complexContentBase(child, ctx, ct)
	if err != nil {
		return complexType{}, err
	}
	if mixed && !base.Mixed {
		return complexType{}, schemaCompile(ErrSchemaContentModel, "complexContent mixed derivation requires mixed base")
	}
	if err := validateComplexContentDerivationChildren(child); err != nil {
		return complexType{}, err
	}
	ct.Base = typeID{Kind: typeComplex, ID: uint32(baseID)}
	if child.Name.Local == xsdElemExtension {
		return c.compileComplexContentExtension(child, ctx, ct, baseID, base, mixed)
	}
	return c.compileComplexContentRestriction(child, ctx, ct, base, mixed)
}

func (c *compiler) complexContentBase(child *rawNode, ctx *schemaContext, ct complexType) (complexTypeID, complexType, error) {
	baseLex, ok := child.attr(xsdAttrBase)
	if !ok {
		return noComplexType, complexType{}, schemaCompile(ErrSchemaReference, "complexContent "+child.Name.Local+" missing base")
	}
	baseQName, err := c.resolveQNameChecked(child, ctx, baseLex)
	if err != nil {
		return noComplexType, complexType{}, err
	}
	if c.compilingComplex[baseQName] && !c.isAnonymousComplexName(ct.Name) {
		return noComplexType, complexType{}, schemaCompile(ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(baseQName))
	}
	baseID, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return noComplexType, complexType{}, err
	}
	return baseID, c.rt.ComplexTypes[baseID], nil
}

func (c *compiler) compileComplexContentExtension(child *rawNode, ctx *schemaContext, ct complexType, baseID complexTypeID, base complexType, mixed bool) (complexType, error) {
	if base.SimpleValue {
		return c.compileSimpleValueComplexExtension(child, ctx, ct, base, mixed)
	}
	if base.Final&blockExtension != 0 {
		return complexType{}, schemaCompile(ErrSchemaReference, "base complex type final blocks extension")
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
	ct.Mixed = base.Mixed || mixed
	return ct, nil
}

func (c *compiler) compileSimpleValueComplexExtension(child *rawNode, ctx *schemaContext, ct, base complexType, mixed bool) (complexType, error) {
	if firstModelChild(child) != nil {
		return complexType{}, schemaCompile(ErrSchemaContentModel, "complexContent extension cannot add particles to simple content")
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
	ct.SimpleValue = true
	ct.Mixed = mixed
	return ct, nil
}

func (c *compiler) compileComplexExtensionModel(modelNode *rawNode, ctx *schemaContext, baseID complexTypeID, base complexType, mixed bool) (contentModelID, error) {
	if baseID != c.rt.Builtin.AnyType && base.Mixed && !mixed {
		return noContentModel, schemaCompile(ErrSchemaContentModel, "complexContent extension cannot drop mixed base content")
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return noContentModel, err
	}
	if modelNode.Name.Local == xsdElemAll && !c.modelHasNoParticles(base.Content) {
		return noContentModel, schemaCompile(ErrSchemaContentModel, "complexContent extension cannot use xs:all")
	}
	if base.Content != noContentModel {
		baseModel := c.rt.Models[base.Content]
		if baseModel.Kind == modelAll && len(baseModel.Particles) != 0 {
			return noContentModel, schemaCompile(ErrSchemaContentModel, "complexContent extension cannot add particles to xs:all base")
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
		return complexType{}, schemaCompile(ErrSchemaReference, "base complex type final blocks restriction")
	}
	if base.SimpleValue {
		return complexType{}, schemaCompile(ErrSchemaContentModel, "complexContent restriction base cannot have simple content")
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
	ct.Mixed = mixed
	return ct, nil
}

func (c *compiler) compileComplexRestrictionModel(child *rawNode, ctx *schemaContext, ct complexType) (contentModelID, error) {
	modelNode := firstModelChild(child)
	if modelNode == nil {
		return c.addModel(contentModel{Kind: modelEmpty, Mixed: ct.Mixed})
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return noContentModel, err
	}
	return c.compileModel(modelNode, ctx)
}

func validateComplexContentChildren(n *rawNode) error {
	seenDerivation := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "complexContent annotation must be first")
			}
		case xsdElemExtension, xsdElemRestriction:
			if seenDerivation {
				return schemaCompile(ErrSchemaContentModel, "complexContent can contain one derivation")
			}
			seenDerivation = true
			seenNonAnnotation = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid complexContent child "+child.Name.Local)
		}
	}
	return nil
}

func validateSimpleContentChildren(n *rawNode) error {
	seenDerivation := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "simpleContent annotation must be first")
			}
		case xsdElemExtension, xsdElemRestriction:
			if seenDerivation {
				return schemaCompile(ErrSchemaContentModel, "simpleContent can contain one derivation")
			}
			seenDerivation = true
			seenNonAnnotation = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid simpleContent child "+child.Name.Local)
		}
	}
	return nil
}

func validateSimpleContentDerivationChildren(n *rawNode) error {
	seenNonAnnotation := false
	seenSimpleType := false
	seenAttribute := false
	seenAnyAttribute := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
		case xsdElemSimpleType:
			if n.Name.Local != xsdElemRestriction {
				return schemaCompile(ErrSchemaContentModel, "simpleContent extension cannot contain simpleType")
			}
			if seenSimpleType || seenAttribute || seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, "simpleContent simpleType is out of order")
			}
			seenSimpleType = true
			seenNonAnnotation = true
		case xsdElemAttribute, xsdElemAttributeGroup:
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, "simpleContent attribute is out of order")
			}
			seenAttribute = true
			seenNonAnnotation = true
		case xsdElemAnyAttribute:
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, "simpleContent can contain at most one anyAttribute")
			}
			seenAnyAttribute = true
			seenNonAnnotation = true
		default:
			if n.Name.Local == xsdElemRestriction && isFacetNode(child.Name.Local) {
				if seenAttribute || seenAnyAttribute {
					return schemaCompile(ErrSchemaContentModel, "simpleContent facet is out of order")
				}
				seenNonAnnotation = true
				continue
			}
			return schemaCompile(ErrSchemaContentModel, "invalid simpleContent "+n.Name.Local+" child "+child.Name.Local)
		}
	}
	return nil
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

func (c *compiler) compileSimpleContent(n *rawNode, ctx *schemaContext, ct complexType) (complexType, error) {
	if err := validateSimpleContentChildren(n); err != nil {
		return complexType{}, err
	}
	child := simpleContentDerivationChild(n)
	if child == nil {
		return complexType{}, schemaCompile(ErrSchemaContentModel, "simpleContent missing extension or restriction")
	}
	if err := validateSimpleContentDerivationChildren(child); err != nil {
		return complexType{}, err
	}
	baseLex, ok := child.attr(xsdAttrBase)
	if !ok {
		return complexType{}, schemaCompile(ErrSchemaReference, "simpleContent "+child.Name.Local+" missing base")
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
		ct, textType, err = c.compileSimpleContentComplexBase(child, baseQName, ct)
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
	ct.SimpleValue = true
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
		return complexType{}, noSimpleType, err
	}
	if child.Name.Local == xsdElemRestriction {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaContentModel, "simpleContent restriction base must be complex type")
	}
	if c.rt.SimpleTypes[simpleID].Final&blockExtension != 0 {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaReference, "base simple type final blocks extension")
	}
	ct.Base = typeID{Kind: typeSimple, ID: uint32(simpleID)}
	return ct, simpleID, nil
}

func (c *compiler) compileSimpleContentComplexBase(child *rawNode, baseQName qName, ct complexType) (complexType, simpleTypeID, error) {
	if c.compilingComplex[baseQName] && !c.isAnonymousComplexName(ct.Name) {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(baseQName))
	}
	if !c.complexTypeQNameKnown(baseQName) {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaReference, "simpleContent base must be simple or simple-content complex type")
	}
	baseComplex, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return complexType{}, noSimpleType, err
	}
	base := c.rt.ComplexTypes[baseComplex]
	if !base.SimpleValue {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaContentModel, "simpleContent base must have simple content")
	}
	if child.Name.Local == xsdElemExtension && base.Final&blockExtension != 0 {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaReference, "base complex type final blocks extension")
	}
	if child.Name.Local == xsdElemRestriction && base.Final&blockRestriction != 0 {
		return complexType{}, noSimpleType, schemaCompile(ErrSchemaReference, "base complex type final blocks restriction")
	}
	ct.Base = typeID{Kind: typeComplex, ID: uint32(baseComplex)}
	ct.Attrs = base.Attrs
	return ct, base.TextType, nil
}

func (c *compiler) compileSimpleContentRestrictionType(child *rawNode, ctx *schemaContext, baseTextType simpleTypeID) (simpleTypeID, error) {
	textType := baseTextType
	if stNode := child.firstXS(xsdElemSimpleType); stNode != nil {
		simpleID, err := c.compileAnonymousSimple(stNode, ctx)
		if err != nil {
			return noSimpleType, err
		}
		textType = simpleID
	} else if hasFacetChildren(child) {
		simpleID, err := c.compileSimpleContentFacetRestriction(child, baseTextType)
		if err != nil {
			return noSimpleType, err
		}
		textType = simpleID
	}
	if !c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(textType)}, typeID{Kind: typeSimple, ID: uint32(baseTextType)}) {
		return noSimpleType, schemaCompile(ErrSchemaContentModel, "simpleContent restriction type is not derived from base")
	}
	return textType, nil
}

func hasFacetChildren(n *rawNode) bool {
	for _, child := range n.Children {
		if child.Name.Space == xsdNamespaceURI && isFacetNode(child.Name.Local) {
			return true
		}
	}
	return false
}

func (c *compiler) compileSimpleContentFacetRestriction(n *rawNode, baseID simpleTypeID) (simpleTypeID, error) {
	if baseID == c.rt.Builtin.AnySimpleType {
		return noSimpleType, schemaCompile(ErrSchemaReference, "simple type cannot restrict xs:anySimpleType")
	}
	base := c.rt.SimpleTypes[baseID]
	if base.Final&blockRestriction != 0 {
		return noSimpleType, schemaCompile(ErrSchemaReference, "base simple type final blocks restriction")
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
	if err = c.compileFacets(facetChildrenNode(n), &st, baseID, baseID); err != nil {
		return noSimpleType, err
	}
	id, err := nextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return noSimpleType, err
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, st)
	return id, nil
}

func facetChildrenNode(n *rawNode) *rawNode {
	out := *n
	out.Children = nil
	for _, child := range n.Children {
		if child.Name.Space == xsdNamespaceURI && isFacetNode(child.Name.Local) {
			out.Children = append(out.Children, child)
		}
	}
	return &out
}

func firstModelChild(n *rawNode) *rawNode {
	children := modelChildren(n)
	if len(children) == 0 {
		return nil
	}
	return children[0]
}

func validateComplexContentDerivationChildren(n *rawNode) error {
	seenModel := false
	seenAttribute := false
	seenAnyAttribute := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
		case xsdElemSequence, xsdElemChoice, xsdElemAll, xsdElemGroup:
			if seenModel || seenAttribute || seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" model group is out of order")
			}
			seenModel = true
			seenNonAnnotation = true
		case xsdElemAttribute, xsdElemAttributeGroup:
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" attribute is out of order")
			}
			seenAttribute = true
			seenNonAnnotation = true
		case xsdElemAnyAttribute:
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" can contain at most one anyAttribute")
			}
			seenAnyAttribute = true
			seenNonAnnotation = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid complexContent child "+child.Name.Local)
		}
	}
	return nil
}
