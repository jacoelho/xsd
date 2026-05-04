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
	id := complexTypeID(len(c.rt.ComplexTypes))
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, Base: typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}})
	c.complexDone[q] = id
	c.rt.GlobalTypes[q] = typeID{Kind: typeComplex, ID: uint32(id)}
	ct, err := c.compileComplexType(raw.node, raw.ctx, q)
	if err != nil {
		return noComplexType, err
	}
	ct.Name = q
	ct.Block = complexBlockMaskWithDefault(raw.node, raw.ctx.blockDefault)
	ct.Final = derivationMaskWithDefault(raw.node, "final", raw.ctx.finalDefault)
	c.rt.ComplexTypes[id] = ct
	return id, nil
}

func (c *compiler) compileAnonymousComplex(n *rawNode, ctx *schemaContext) (complexTypeID, error) {
	if _, ok := n.attr("name"); ok {
		return noComplexType, schemaCompile(ErrSchemaInvalidAttribute, "local complexType cannot have name")
	}
	q := c.rt.Names.InternQName("", fmt.Sprintf("$complex%d", len(c.rt.ComplexTypes)))
	id := complexTypeID(len(c.rt.ComplexTypes))
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, Base: typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}})
	ct, err := c.compileComplexType(n, ctx, q)
	if err != nil {
		return noComplexType, err
	}
	ct.Name = q
	ct.Final = derivationMaskWithDefault(n, "final", ctx.finalDefault)
	c.rt.ComplexTypes[id] = ct
	return id, nil
}

func (c *compiler) isAnonymousComplexName(q qName) bool {
	return c.rt.Names.Namespace(q.Namespace) == "" && strings.HasPrefix(c.rt.Names.Local(q.Local), "$complex")
}

func schemaBoolAttr(n *rawNode, name string, def bool) (bool, error) {
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
	case "qualified":
		return true, nil
	case "unqualified":
		return false, nil
	default:
		return false, schemaCompile(ErrSchemaInvalidAttribute, "invalid "+name+" value "+v)
	}
}

func validateKnownAttributes(n *rawNode, label string, allowed map[string]bool) error {
	for _, attr := range n.Attr {
		if isNamespaceAttr(attr) || attr.Name.Space != "" {
			continue
		}
		if !allowed[attr.Name.Local] {
			return schemaCompile(ErrSchemaInvalidAttribute, label+" cannot have attribute "+attr.Name.Local)
		}
	}
	return nil
}

func validateComplexTypeDerivationAttrs(n *rawNode) error {
	for _, attr := range []string{"block", "final"} {
		v, ok := n.attr(attr)
		if !ok {
			continue
		}
		fieldCount := 0
		for range strings.FieldsSeq(v) {
			fieldCount++
		}
		i := 0
		for field := range strings.FieldsSeq(v) {
			switch field {
			case "#all":
				if fieldCount != 1 {
					return schemaCompile(ErrSchemaInvalidAttribute, attr+" cannot combine #all with other values")
				}
			case "extension", "restriction":
			case "substitution":
				return schemaCompile(ErrSchemaInvalidAttribute, "complexType "+attr+" cannot contain substitution")
			default:
				return schemaCompile(ErrSchemaInvalidAttribute, "invalid complexType "+attr+" value "+field)
			}
			if field == "#all" && i != 0 {
				return schemaCompile(ErrSchemaInvalidAttribute, attr+" cannot combine #all with other values")
			}
			i++
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
		case "annotation":
			if sawModel || sawAttr || sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType annotation must be first")
			}
		case "simpleContent", "complexContent":
			if sawModel || sawAttr || sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType content model is out of order")
			}
			terminal = true
		case "sequence", "choice", "all", "group":
			if sawModel || sawAttr || sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType model group is out of order")
			}
			sawModel = true
		case "attribute", "attributeGroup":
			if sawAnyAttr {
				return schemaCompile(ErrSchemaContentModel, "complexType attribute is out of order")
			}
			sawAttr = true
		case "anyAttribute":
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
	if err := validateComplexTypeDerivationAttrs(n); err != nil {
		return complexType{}, err
	}
	mixed, err := schemaBoolAttr(n, "mixed", false)
	if err != nil {
		return complexType{}, err
	}
	abstract, err := schemaBoolAttr(n, "abstract", false)
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
		Block:      complexBlockMaskWithDefault(n, ctx.blockDefault),
	}
	if cc := n.firstXS("complexContent"); cc != nil {
		return c.compileComplexContent(cc, ctx, ct)
	}
	if sc := n.firstXS("simpleContent"); sc != nil {
		return c.compileSimpleContent(sc, ctx, ct)
	}
	for _, child := range n.xsContentChildren() {
		switch child.Name.Local {
		case "sequence", "choice", "all", "group":
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
		ct.Content = c.addModel(contentModel{Kind: modelEmpty, Mixed: mixed})
	}
	attrs, err := c.compileAttributeUses(n, ctx, nil, noWildcard, false)
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
	mixed, err := schemaBoolAttr(n, "mixed", ct.Mixed)
	if err != nil {
		return complexType{}, err
	}
	for _, child := range n.xsContentChildren() {
		if child.Name.Local != "extension" && child.Name.Local != "restriction" {
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
	if child.Name.Local == "extension" {
		return c.compileComplexContentExtension(child, ctx, ct, baseID, base, mixed)
	}
	return c.compileComplexContentRestriction(child, ctx, ct, base, mixed)
}

func (c *compiler) complexContentBase(child *rawNode, ctx *schemaContext, ct complexType) (complexTypeID, complexType, error) {
	baseLex, ok := child.attr("base")
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
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, false)
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
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, false)
	if err != nil {
		return complexType{}, err
	}
	ct.Derivation = derivationExtension
	ct.Content = c.addModel(contentModel{Kind: modelEmpty})
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
	if modelNode.Name.Local == "all" && !c.modelHasNoParticles(base.Content) {
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
	return c.extendSequence(base.Content, ext), nil
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
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, true)
	if err != nil {
		return complexType{}, err
	}
	if err := c.validateContentRestriction(base.Content, ct.Content); err != nil {
		return complexType{}, err
	}
	ct.CountLimits = c.restrictionCountLimits(base.Content, ct.Content)
	ct.Attrs = attrs
	ct.Mixed = mixed
	return ct, nil
}

func (c *compiler) compileComplexRestrictionModel(child *rawNode, ctx *schemaContext, ct complexType) (contentModelID, error) {
	modelNode := firstModelChild(child)
	if modelNode == nil {
		return c.addModel(contentModel{Kind: modelEmpty, Mixed: ct.Mixed}), nil
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
		case "annotation":
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "complexContent annotation must be first")
			}
		case "extension", "restriction":
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
		case "annotation":
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "simpleContent annotation must be first")
			}
		case "extension", "restriction":
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
		case "annotation":
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
		case "simpleType":
			if n.Name.Local != "restriction" {
				return schemaCompile(ErrSchemaContentModel, "simpleContent extension cannot contain simpleType")
			}
			if seenSimpleType || seenAttribute || seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, "simpleContent simpleType is out of order")
			}
			seenSimpleType = true
			seenNonAnnotation = true
		case "attribute", "attributeGroup":
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, "simpleContent attribute is out of order")
			}
			seenAttribute = true
			seenNonAnnotation = true
		case "anyAttribute":
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, "simpleContent can contain at most one anyAttribute")
			}
			seenAnyAttribute = true
			seenNonAnnotation = true
		default:
			if n.Name.Local == "restriction" && isFacetNode(child.Name.Local) {
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
	case "length", "minLength", "maxLength", "totalDigits", "fractionDigits",
		"minInclusive", "maxInclusive", "minExclusive", "maxExclusive",
		"enumeration", "pattern", "whiteSpace":
		return true
	default:
		return false
	}
}

func (c *compiler) compileSimpleContent(n *rawNode, ctx *schemaContext, ct complexType) (complexType, error) {
	if err := validateSimpleContentChildren(n); err != nil {
		return complexType{}, err
	}
	for _, child := range n.xsContentChildren() {
		if child.Name.Local != "extension" && child.Name.Local != "restriction" {
			continue
		}
		if err := validateSimpleContentDerivationChildren(child); err != nil {
			return complexType{}, err
		}
		baseLex, ok := child.attr("base")
		if !ok {
			return complexType{}, schemaCompile(ErrSchemaReference, "simpleContent "+child.Name.Local+" missing base")
		}
		baseQName, err := c.resolveQNameChecked(child, ctx, baseLex)
		if err != nil {
			return complexType{}, err
		}
		var textType simpleTypeID
		if simpleID, simpleErr := c.compileSimpleByQName(baseQName); simpleErr == nil {
			if child.Name.Local == "restriction" {
				return complexType{}, schemaCompile(ErrSchemaContentModel, "simpleContent restriction base must be complex type")
			}
			textType = simpleID
			ct.Base = typeID{Kind: typeSimple, ID: uint32(simpleID)}
		} else {
			if c.compilingComplex[baseQName] && !c.isAnonymousComplexName(ct.Name) {
				return complexType{}, schemaCompile(ErrSchemaReference, "cyclic complex type "+c.rt.Names.Format(baseQName))
			}
			baseComplex, complexErr := c.compileComplexByQName(baseQName)
			if complexErr != nil {
				return complexType{}, schemaCompile(ErrSchemaReference, "simpleContent base must be simple or simple-content complex type")
			}
			base := c.rt.ComplexTypes[baseComplex]
			if !base.SimpleValue {
				return complexType{}, schemaCompile(ErrSchemaContentModel, "simpleContent base must have simple content")
			}
			if child.Name.Local == "extension" && base.Final&blockExtension != 0 {
				return complexType{}, schemaCompile(ErrSchemaReference, "base complex type final blocks extension")
			}
			if child.Name.Local == "restriction" && base.Final&blockRestriction != 0 {
				return complexType{}, schemaCompile(ErrSchemaReference, "base complex type final blocks restriction")
			}
			textType = base.TextType
			ct.Base = typeID{Kind: typeComplex, ID: uint32(baseComplex)}
			ct.Attrs = base.Attrs
		}
		if child.Name.Local == "restriction" {
			baseTextType := textType
			if stNode := child.firstXS("simpleType"); stNode != nil {
				simpleID, simpleErr := c.compileAnonymousSimple(stNode, ctx)
				if simpleErr != nil {
					return complexType{}, simpleErr
				}
				textType = simpleID
			} else if hasFacetChildren(child) {
				simpleID, simpleErr := c.compileSimpleContentFacetRestriction(child, baseTextType)
				if simpleErr != nil {
					return complexType{}, simpleErr
				}
				textType = simpleID
			}
			if !c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(textType)}, typeID{Kind: typeSimple, ID: uint32(baseTextType)}) {
				return complexType{}, schemaCompile(ErrSchemaContentModel, "simpleContent restriction type is not derived from base")
			}
		}
		inheritedUses, inheritedWildcard := c.attrUsesAndWildcard(ct.Attrs)
		attrs, err := c.compileAttributeUses(child, ctx, inheritedUses, inheritedWildcard, child.Name.Local == "restriction")
		if err != nil {
			return complexType{}, err
		}
		ct.Attrs = attrs
		ct.Content = c.addModel(contentModel{Kind: modelEmpty})
		ct.TextType = textType
		ct.SimpleValue = true
		if child.Name.Local == "extension" {
			ct.Derivation = derivationExtension
		} else {
			ct.Derivation = derivationRestriction
		}
		return ct, nil
	}
	return complexType{}, schemaCompile(ErrSchemaContentModel, "simpleContent missing extension or restriction")
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
	q := c.rt.Names.InternQName("", fmt.Sprintf("$simple%d", len(c.rt.SimpleTypes)))
	st := base
	st.Name = q
	st.Base = baseID
	st.Final = 0
	st.Facets = cloneFacetSet(base.Facets)
	st.Union = slices.Clone(base.Union)
	if err := c.compileFacets(facetChildrenNode(n), &st, baseID); err != nil {
		return noSimpleType, err
	}
	id := simpleTypeID(len(c.rt.SimpleTypes))
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
		case "annotation":
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
		case "sequence", "choice", "all", "group":
			if seenModel || seenAttribute || seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" model group is out of order")
			}
			seenModel = true
			seenNonAnnotation = true
		case "attribute", "attributeGroup":
			if seenAnyAttribute {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" attribute is out of order")
			}
			seenAttribute = true
			seenNonAnnotation = true
		case "anyAttribute":
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
