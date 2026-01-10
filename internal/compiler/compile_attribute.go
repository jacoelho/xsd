package compiler

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *Compiler) mergeAttributes(ct *types.ComplexType, chain []*grammar.CompiledType) []*grammar.CompiledAttribute {
	// later types override earlier ones (restriction) or add to them (extension)
	attrMap := make(map[types.QName]*grammar.CompiledAttribute)

	// process from base to derived (reverse order)
	for i := len(chain) - 1; i >= 0; i-- {
		compiledType := chain[i]
		if compiledType.Kind != grammar.TypeKindComplex {
			continue
		}
		origCT, ok := compiledType.Original.(*types.ComplexType)
		if !ok {
			continue
		}

		c.collectAttributesFromComplexType(origCT, attrMap)
	}

	result := make([]*grammar.CompiledAttribute, 0, len(attrMap))
	for _, attr := range attrMap {
		result = append(result, attr)
	}
	return result
}

// collectAttributesFromComplexType collects attributes from all sources in a complex type:
// - Direct attributes on the type
// - Attributes from SimpleContent extension/restriction
// - Attributes from ComplexContent extension/restriction
// - Attributes from attribute groups
//
// Per XSD 1.0 spec section 3.4.2, "prohibited" attribute uses are NOT included in
// the {attribute uses} property of the complex type. The XSD 1.0 W3C tests treat
// use="prohibited" with fixed as a valid use, so we keep those for schemacheck.
func (c *Compiler) collectAttributesFromComplexType(ct *types.ComplexType, attrMap map[types.QName]*grammar.CompiledAttribute) {
	c.collectAttributes(ct.Attributes(), ct.AttrGroups, attrMap, false)

	// 3. Check content for extension/restriction attributes
	content := ct.Content()
	if ext := content.ExtensionDef(); ext != nil {
		c.collectAttributes(ext.Attributes, ext.AttrGroups, attrMap, false)
	}
	if restr := content.RestrictionDef(); restr != nil {
		c.collectAttributes(restr.Attributes, restr.AttrGroups, attrMap, true)
	}
}

func (c *Compiler) collectAttributes(attrs []*types.AttributeDecl, attrGroups []types.QName, attrMap map[types.QName]*grammar.CompiledAttribute, isRestriction bool) {
	for _, attr := range attrs {
		if !shouldIncludeAttribute(attr) {
			if isRestriction {
				delete(attrMap, c.effectiveAttributeQName(attr))
			}
			continue
		}
		c.addCompiledAttribute(attr, attrMap)
	}

	for _, agRef := range attrGroups {
		if ag, ok := c.schema.AttributeGroups[agRef]; ok {
			c.mergeAttributesFromGroup(ag, attrMap)
		}
	}
}

// addCompiledAttribute adds a single attribute to the map
func (c *Compiler) addCompiledAttribute(attr *types.AttributeDecl, attrMap map[types.QName]*grammar.CompiledAttribute) {
	effectiveQName := c.effectiveAttributeQName(attr)

	compiled := &grammar.CompiledAttribute{
		QName:    effectiveQName,
		Original: attr,
		Use:      attr.Use,
		Default:  attr.Default,
		Fixed:    attr.Fixed,
		HasFixed: attr.HasFixed,
	}

	// get the type either from the attribute itself or by resolving the reference
	attrType := attr.Type
	if attr.IsReference {
		// attribute reference - look up the global attribute declaration
		if globalAttr, ok := c.schema.AttributeDecls[attr.Name]; ok {
			if attrType == nil {
				attrType = globalAttr.Type
			}
			// also inherit default/fixed from global if not set locally
			if compiled.Default == "" {
				compiled.Default = globalAttr.Default
			}
			if !compiled.HasFixed && globalAttr.HasFixed {
				compiled.Fixed = globalAttr.Fixed
				compiled.HasFixed = globalAttr.HasFixed
			}
		}
	}

	if attrType != nil {
		attrTypeCompiled, err := c.compileType(attrType.Name(), attrType)
		if err == nil {
			compiled.Type = attrTypeCompiled
		}
	}
	attrMap[effectiveQName] = compiled
}

// effectiveAttributeQName computes the namespace-qualified name for an attribute
// based on its form attribute and the schema's attributeFormDefault.
func (c *Compiler) effectiveAttributeQName(attr *types.AttributeDecl) types.QName {
	// references use the namespace of the referenced global attribute
	if attr.IsReference {
		return attr.Name
	}

	if !c.isAttributeQualified(attr) {
		return types.QName{Namespace: "", Local: attr.Name.Local}
	}

	ns := c.attributeNamespace(attr)
	return types.QName{Namespace: ns, Local: attr.Name.Local}
}

func shouldIncludeAttribute(attr *types.AttributeDecl) bool {
	return attr.Use != types.Prohibited || attr.HasFixed
}

func (c *Compiler) isAttributeQualified(attr *types.AttributeDecl) bool {
	switch attr.Form {
	case types.FormQualified:
		return true
	case types.FormUnqualified:
		return false
	default:
		return c.grammar.AttributeFormDefault == parser.Qualified
	}
}

func (c *Compiler) attributeNamespace(attr *types.AttributeDecl) types.NamespaceURI {
	ns := c.schema.TargetNamespace
	if !attr.SourceNamespace.IsEmpty() {
		ns = attr.SourceNamespace
	}
	return ns
}

func (c *Compiler) mergeAttributesFromGroup(ag *types.AttributeGroup, attrMap map[types.QName]*grammar.CompiledAttribute) {
	// iterative approach with work queue
	// use pointer-based cycle detection since the same QName can refer to different
	// attribute group instances (redefined vs original in redefine context)
	visited := make(map[*types.AttributeGroup]bool)
	queue := []*types.AttributeGroup{ag}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, attr := range current.Attributes {
			if !shouldIncludeAttribute(attr) {
				continue
			}
			c.addCompiledAttribute(attr, attrMap)
		}

		for _, agRef := range current.AttrGroups {
			if refAG, ok := c.schema.AttributeGroups[agRef]; ok {
				queue = append(queue, refAG)
			}
		}
	}
}

func (c *Compiler) mergeAnyAttribute(chain []*grammar.CompiledType) *types.AnyAttribute {
	// for extension: union of namespace constraints with base
	// for restriction: derived type's anyAttribute replaces base's (no inheritance)
	// within a single type, anyAttribute from the type and attribute groups are intersected
	// process from derived (first) to base (last) in chain
	derivedCT, typeAnyAttrs := c.extractDerivedWildcards(chain)
	if derivedCT == nil {
		return nil
	}

	switch c.getDerivationKind(derivedCT) {
	case restrictionDerivation:
		return c.mergeAnyAttributeRestriction(typeAnyAttrs)
	case extensionDerivation:
		return c.mergeAnyAttributeExtension(typeAnyAttrs, chain)
	default:
		return c.mergeAnyAttributeFallback(typeAnyAttrs, chain)
	}
}

type derivationKind int

const (
	unknownDerivation derivationKind = iota
	restrictionDerivation
	extensionDerivation
)

func (c *Compiler) extractDerivedWildcards(chain []*grammar.CompiledType) (*types.ComplexType, []*types.AnyAttribute) {
	if len(chain) == 0 {
		return nil, nil
	}
	derivedCT := chain[0]
	if derivedCT.Kind != grammar.TypeKindComplex {
		return nil, nil
	}
	origCT, ok := derivedCT.Original.(*types.ComplexType)
	if !ok {
		return nil, nil
	}
	return origCT, c.collectTypeAnyAttributes(origCT)
}

func (c *Compiler) getDerivationKind(ct *types.ComplexType) derivationKind {
	if cc, ok := ct.Content().(*types.ComplexContent); ok {
		if cc.Restriction != nil {
			return restrictionDerivation
		}
		if cc.Extension != nil {
			return extensionDerivation
		}
	}
	if sc, ok := ct.Content().(*types.SimpleContent); ok {
		if sc.Restriction != nil {
			return restrictionDerivation
		}
		if sc.Extension != nil {
			return extensionDerivation
		}
	}
	return unknownDerivation
}

func (c *Compiler) mergeAnyAttributeRestriction(typeAnyAttrs []*types.AnyAttribute) *types.AnyAttribute {
	if len(typeAnyAttrs) == 0 {
		return nil
	}
	return c.intersectWildcards(typeAnyAttrs)
}

func (c *Compiler) mergeAnyAttributeExtension(typeAnyAttrs []*types.AnyAttribute, chain []*grammar.CompiledType) *types.AnyAttribute {
	derivedWildcard := c.intersectWildcards(typeAnyAttrs)
	baseWildcard := c.mergeAnyAttributeBase(chain)
	if derivedWildcard == nil {
		return baseWildcard
	}
	if baseWildcard == nil {
		return derivedWildcard
	}
	return types.UnionAnyAttribute(derivedWildcard, baseWildcard)
}

func (c *Compiler) mergeAnyAttributeFallback(typeAnyAttrs []*types.AnyAttribute, chain []*grammar.CompiledType) *types.AnyAttribute {
	derivedWildcard := c.intersectWildcards(typeAnyAttrs)
	if derivedWildcard != nil {
		return derivedWildcard
	}
	return c.mergeAnyAttributeBase(chain)
}

func (c *Compiler) mergeAnyAttributeBase(chain []*grammar.CompiledType) *types.AnyAttribute {
	if len(chain) <= 1 {
		return nil
	}
	return c.mergeAnyAttribute(chain[1:])
}

func (c *Compiler) intersectWildcards(wildcards []*types.AnyAttribute) *types.AnyAttribute {
	if len(wildcards) == 0 {
		return nil
	}
	result := wildcards[0]
	for i := 1; i < len(wildcards); i++ {
		result = types.IntersectAnyAttribute(result, wildcards[i])
		if result == nil {
			return nil
		}
	}
	return result
}

func (c *Compiler) collectTypeAnyAttributes(ct *types.ComplexType) []*types.AnyAttribute {
	var result []*types.AnyAttribute

	if anyAttr := ct.AnyAttribute(); anyAttr != nil {
		result = append(result, anyAttr)
	}

	result = append(result, c.collectAnyAttributeFromGroups(ct.AttrGroups)...)

	// also check for anyAttribute in ComplexContent extension/restriction
	if cc, ok := ct.Content().(*types.ComplexContent); ok {
		if cc.Extension != nil {
			if cc.Extension.AnyAttribute != nil {
				result = append(result, cc.Extension.AnyAttribute)
			}
			result = append(result, c.collectAnyAttributeFromGroups(cc.Extension.AttrGroups)...)
		}
		if cc.Restriction != nil {
			if cc.Restriction.AnyAttribute != nil {
				result = append(result, cc.Restriction.AnyAttribute)
			}
			result = append(result, c.collectAnyAttributeFromGroups(cc.Restriction.AttrGroups)...)
		}
	}

	// also check SimpleContent
	if sc, ok := ct.Content().(*types.SimpleContent); ok {
		if sc.Extension != nil {
			if sc.Extension.AnyAttribute != nil {
				result = append(result, sc.Extension.AnyAttribute)
			}
			result = append(result, c.collectAnyAttributeFromGroups(sc.Extension.AttrGroups)...)
		}
		if sc.Restriction != nil {
			if sc.Restriction.AnyAttribute != nil {
				result = append(result, sc.Restriction.AnyAttribute)
			}
			result = append(result, c.collectAnyAttributeFromGroups(sc.Restriction.AttrGroups)...)
		}
	}

	return result
}

// collectAnyAttributeFromGroups collects anyAttribute from attribute groups recursively
func (c *Compiler) collectAnyAttributeFromGroups(agRefs []types.QName) []*types.AnyAttribute {
	var result []*types.AnyAttribute
	visited := make(map[types.QName]bool)

	var collect func(refs []types.QName)
	collect = func(refs []types.QName) {
		for _, agRef := range refs {
			if visited[agRef] {
				continue
			}
			visited[agRef] = true

			ag, ok := c.schema.AttributeGroups[agRef]
			if !ok {
				continue
			}

			if ag.AnyAttribute != nil {
				result = append(result, ag.AnyAttribute)
			}

			// recursively process nested attribute groups
			collect(ag.AttrGroups)
		}
	}

	collect(agRefs)
	return result
}
