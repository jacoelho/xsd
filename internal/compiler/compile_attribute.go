package compiler

import (
	"fmt"
	"maps"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *Compiler) mergeAttributes(chain []*grammar.CompiledType) ([]*grammar.CompiledAttribute, error) {
	// later types override earlier ones (restriction) or add to them (extension)
	attrMap := make(map[types.QName]*grammar.CompiledAttribute)

	// process from base to derived (reverse order)
	for i := len(chain) - 1; i >= 0; i-- {
		compiledType := chain[i]
		if compiledType.Kind != grammar.TypeKindComplex {
			continue
		}
		originalComplexType, ok := compiledType.Original.(*types.ComplexType)
		if !ok {
			continue
		}

		if err := c.collectAttributesFromComplexType(originalComplexType, attrMap); err != nil {
			return nil, err
		}
	}

	result := make([]*grammar.CompiledAttribute, 0, len(attrMap))
	for _, attr := range attrMap {
		result = append(result, attr)
	}
	return result, nil
}

type attributeCollectionMode int

const (
	attributeCollectionMerge attributeCollectionMode = iota
	attributeCollectionRestriction
)

func shouldIncludeAttribute(attr *types.AttributeDecl) bool {
	return attr.Use != types.Prohibited || attr.HasFixed
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
func (c *Compiler) collectAttributesFromComplexType(complexType *types.ComplexType, attrMap map[types.QName]*grammar.CompiledAttribute) error {
	if err := c.collectAttributes(complexType.Attributes(), complexType.AttrGroups, attrMap, attributeCollectionMerge); err != nil {
		return err
	}

	// 3. Check content for extension/restriction attributes
	content := complexType.Content()
	if ext := content.ExtensionDef(); ext != nil {
		if err := c.collectAttributes(ext.Attributes, ext.AttrGroups, attrMap, attributeCollectionMerge); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := c.collectAttributes(restr.Attributes, restr.AttrGroups, attrMap, attributeCollectionRestriction); err != nil {
			return err
		}
	}
	return nil
}

func (c *Compiler) collectAttributes(attrs []*types.AttributeDecl, attrGroups []types.QName, attrMap map[types.QName]*grammar.CompiledAttribute, mode attributeCollectionMode) error {
	for _, attr := range attrs {
		if !shouldIncludeAttribute(attr) {
			if mode == attributeCollectionRestriction {
				delete(attrMap, c.effectiveAttributeQName(attr))
			}
			continue
		}
		if err := c.addCompiledAttribute(attr, attrMap); err != nil {
			return err
		}
	}

	for _, agRef := range attrGroups {
		if ag, ok := c.schema.AttributeGroups[agRef]; ok {
			if err := c.mergeAttributesFromGroup(ag, attrMap); err != nil {
				return err
			}
		}
	}
	return nil
}

// addCompiledAttribute adds a single attribute to the map
func (c *Compiler) addCompiledAttribute(attr *types.AttributeDecl, attrMap map[types.QName]*grammar.CompiledAttribute) error {
	effectiveQName := c.effectiveAttributeQName(attr)
	effectiveAttr := attr
	ensureClone := func() *types.AttributeDecl {
		if effectiveAttr != attr {
			return effectiveAttr
		}
		clone := *attr
		effectiveAttr = &clone
		return effectiveAttr
	}

	compiled := &grammar.CompiledAttribute{
		QName:      effectiveQName,
		Use:        attr.Use,
		Default:    attr.Default,
		HasDefault: attr.HasDefault,
		Fixed:      attr.Fixed,
		HasFixed:   attr.HasFixed,
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
			if !compiled.HasDefault && globalAttr.HasDefault {
				compiled.Default = globalAttr.Default
				compiled.HasDefault = true
				if attr.DefaultContext == nil && globalAttr.DefaultContext != nil {
					ensureClone().DefaultContext = maps.Clone(globalAttr.DefaultContext)
				}
			}
			if !compiled.HasFixed && globalAttr.HasFixed {
				compiled.Fixed = globalAttr.Fixed
				compiled.HasFixed = globalAttr.HasFixed
				if attr.FixedContext == nil && globalAttr.FixedContext != nil {
					ensureClone().FixedContext = maps.Clone(globalAttr.FixedContext)
				}
			}
		}
	}

	if compiled.HasDefault && compiled.HasFixed {
		return fmt.Errorf("attribute %s cannot have both default and fixed values", effectiveQName)
	}

	if attrType != nil {
		attrTypeCompiled, err := c.compileType(attrType.Name(), attrType)
		if err != nil {
			return fmt.Errorf("compile attribute %s: %w", effectiveQName, err)
		}
		compiled.Type = attrTypeCompiled
	}
	compiled.Original = effectiveAttr
	attrMap[effectiveQName] = compiled
	return nil
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

func (c *Compiler) mergeAttributesFromGroup(ag *types.AttributeGroup, attrMap map[types.QName]*grammar.CompiledAttribute) error {
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
			if err := c.addCompiledAttribute(attr, attrMap); err != nil {
				return err
			}
		}

		for _, agRef := range current.AttrGroups {
			if refAG, ok := c.schema.AttributeGroups[agRef]; ok {
				queue = append(queue, refAG)
			}
		}
	}
	return nil
}

func (c *Compiler) collectProhibitedAttributes(chain []*grammar.CompiledType) []types.QName {
	if len(chain) == 0 {
		return nil
	}
	seen := make(map[types.QName]bool)
	prohibited := make([]types.QName, 0)

	for _, compiledType := range chain {
		if compiledType == nil || compiledType.Original == nil {
			continue
		}
		complexType, ok := compiledType.Original.(*types.ComplexType)
		if !ok {
			continue
		}

		c.collectProhibitedAttributesFromUses(complexType.Attributes(), complexType.AttrGroups, seen, &prohibited)

		if ext := complexType.Content().ExtensionDef(); ext != nil {
			c.collectProhibitedAttributesFromUses(ext.Attributes, ext.AttrGroups, seen, &prohibited)
		}
		if restr := complexType.Content().RestrictionDef(); restr != nil {
			c.collectProhibitedAttributesFromUses(restr.Attributes, restr.AttrGroups, seen, &prohibited)
		}
	}

	return prohibited
}

func (c *Compiler) collectProhibitedAttributesFromUses(attrs []*types.AttributeDecl, attrGroups []types.QName, seen map[types.QName]bool, out *[]types.QName) {
	for _, attr := range attrs {
		if attr.Use != types.Prohibited {
			continue
		}
		qname := c.effectiveAttributeQName(attr)
		if !seen[qname] {
			*out = append(*out, qname)
			seen[qname] = true
		}
	}
	c.collectProhibitedAttributesFromGroups(attrGroups, seen, out)
}

func (c *Compiler) collectProhibitedAttributesFromGroups(attrGroups []types.QName, seen map[types.QName]bool, out *[]types.QName) {
	if len(attrGroups) == 0 {
		return
	}
	visited := make(map[*types.AttributeGroup]bool)
	queue := make([]*types.AttributeGroup, 0, len(attrGroups))
	for _, ref := range attrGroups {
		if ag, ok := c.schema.AttributeGroups[ref]; ok {
			queue = append(queue, ag)
		}
	}
	for len(queue) > 0 {
		group := queue[0]
		queue = queue[1:]
		if visited[group] {
			continue
		}
		visited[group] = true

		for _, attr := range group.Attributes {
			if attr.Use != types.Prohibited {
				continue
			}
			qname := c.effectiveAttributeQName(attr)
			if !seen[qname] {
				*out = append(*out, qname)
				seen[qname] = true
			}
		}

		for _, ref := range group.AttrGroups {
			if next, ok := c.schema.AttributeGroups[ref]; ok {
				queue = append(queue, next)
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

	switch c.getDerivationKind(derivedCT, chain[0]) {
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

func (c *Compiler) getDerivationKind(complexType *types.ComplexType, compiled *grammar.CompiledType) derivationKind {
	if compiled != nil {
		switch compiled.DerivationMethod {
		case types.DerivationRestriction:
			return restrictionDerivation
		case types.DerivationExtension:
			return extensionDerivation
		}
	}
	if complexType == nil {
		return unknownDerivation
	}
	switch complexType.DerivationMethod {
	case types.DerivationRestriction:
		return restrictionDerivation
	case types.DerivationExtension:
		return extensionDerivation
	}
	if cc, ok := complexType.Content().(*types.ComplexContent); ok {
		if cc.Restriction != nil {
			return restrictionDerivation
		}
		if cc.Extension != nil {
			return extensionDerivation
		}
	}
	if sc, ok := complexType.Content().(*types.SimpleContent); ok {
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

func (c *Compiler) collectTypeAnyAttributes(complexType *types.ComplexType) []*types.AnyAttribute {
	var result []*types.AnyAttribute

	if anyAttr := complexType.AnyAttribute(); anyAttr != nil {
		result = append(result, anyAttr)
	}

	result = append(result, c.collectAnyAttributeFromGroups(complexType.AttrGroups)...)

	if cc, ok := complexType.Content().(*types.ComplexContent); ok {
		result = append(result, c.collectContentAnyAttributes(cc.Extension, cc.Restriction)...)
	}
	if sc, ok := complexType.Content().(*types.SimpleContent); ok {
		result = append(result, c.collectContentAnyAttributes(sc.Extension, sc.Restriction)...)
	}

	return result
}

func (c *Compiler) collectContentAnyAttributes(ext *types.Extension, restr *types.Restriction) []*types.AnyAttribute {
	var result []*types.AnyAttribute
	if ext != nil {
		if ext.AnyAttribute != nil {
			result = append(result, ext.AnyAttribute)
		}
		result = append(result, c.collectAnyAttributeFromGroups(ext.AttrGroups)...)
	}
	if restr != nil {
		if restr.AnyAttribute != nil {
			result = append(result, restr.AnyAttribute)
		}
		result = append(result, c.collectAnyAttributeFromGroups(restr.AttrGroups)...)
	}
	return result
}

// collectAnyAttributeFromGroups collects anyAttribute from attribute groups recursively
func (c *Compiler) collectAnyAttributeFromGroups(agRefs []types.QName) []*types.AnyAttribute {
	var result []*types.AnyAttribute
	visited := make(map[*types.AttributeGroup]bool)

	var collect func(refs []types.QName)
	collect = func(refs []types.QName) {
		for _, agRef := range refs {
			ag, ok := c.schema.AttributeGroups[agRef]
			if !ok {
				continue
			}
			if visited[ag] {
				continue
			}
			visited[ag] = true

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
