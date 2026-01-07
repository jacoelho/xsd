package loader

import (
	"maps"

	"github.com/jacoelho/xsd/internal/grammar"
	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// Compiler transforms a resolved schema into a CompiledSchema (grammar).
// All group references are flattened, derivation chains computed, DFAs built.
type Compiler struct {
	schema  *xsdschema.Schema
	grammar *grammar.CompiledSchema

	// Compiled component cache (by QName for named types)
	types    map[types.QName]*grammar.CompiledType
	elements map[types.QName]*grammar.CompiledElement

	// Cache by type object pointer - handles anonymous types and prevents cycles
	typesByPtr map[types.Type]*grammar.CompiledType

	// Anonymous types (zero QName) - tracked separately for automaton building
	anonymousTypes []*grammar.CompiledType
}

// NewCompiler creates a new compiler for the given schema.
func NewCompiler(schema *xsdschema.Schema) *Compiler {
	return &Compiler{
		schema: schema,
		grammar: &grammar.CompiledSchema{
			TargetNamespace:      schema.TargetNamespace,
			Elements:             make(map[types.QName]*grammar.CompiledElement),
			Types:                make(map[types.QName]*grammar.CompiledType),
			Attributes:           make(map[types.QName]*grammar.CompiledAttribute),
			NotationDecls:        make(map[types.QName]*types.NotationDecl),
			LocalElements:        make(map[types.QName]*grammar.CompiledElement),
			SubstitutionGroups:   make(map[types.QName][]*grammar.CompiledElement),
			ElementFormDefault:   schema.ElementFormDefault,
			AttributeFormDefault: schema.AttributeFormDefault,
			BlockDefault:         schema.BlockDefault,
			FinalDefault:         schema.FinalDefault,
		},
		types:      make(map[types.QName]*grammar.CompiledType),
		elements:   make(map[types.QName]*grammar.CompiledElement),
		typesByPtr: make(map[types.Type]*grammar.CompiledType),
	}
}

// Compile compiles the resolved schema into a CompiledSchema.
func (c *Compiler) Compile() (*grammar.CompiledSchema, error) {
	for qname, typ := range c.schema.TypeDefs {
		if _, err := c.compileType(qname, typ); err != nil {
			return nil, err
		}
	}

	for qname, elem := range c.schema.ElementDecls {
		if _, err := c.compileElement(qname, elem, true); err != nil {
			return nil, err
		}
	}

	// compile top-level attributes (for anyAttribute processContents validation)
	for qname, attr := range c.schema.AttributeDecls {
		if _, err := c.compileTopLevelAttribute(qname, attr); err != nil {
			return nil, err
		}
	}

	// copy notation declarations (notations are not compiled, just copied)
	maps.Copy(c.grammar.NotationDecls, c.schema.NotationDecls)

	c.computeSubstitutionGroups()

	for _, ct := range c.grammar.Types {
		if ct.Kind == grammar.TypeKindComplex && ct.ContentModel != nil && !ct.ContentModel.Empty {
			if err := c.buildAutomaton(ct); err != nil {
				return nil, err
			}
		}
	}

	for _, ct := range c.anonymousTypes {
		if ct.Kind == grammar.TypeKindComplex && ct.ContentModel != nil && !ct.ContentModel.Empty {
			if err := c.buildAutomaton(ct); err != nil {
				return nil, err
			}
		}
	}

	// collect all elements with identity constraints (precomputed for validation)
	c.collectElementsWithConstraints()

	// index all local elements (non-top-level) for XPath evaluation
	c.indexLocalElements()

	return c.grammar, nil
}

// compileElement compiles an element declaration. If isTopLevel is true, the element
// is added to the global Elements map.
func (c *Compiler) compileElement(qname types.QName, elem *types.ElementDecl, isTopLevel bool) (*grammar.CompiledElement, error) {
	if isTopLevel {
		if compiled, ok := c.elements[qname]; ok {
			return compiled, nil
		}
	}

	compiled := &grammar.CompiledElement{
		QName:    qname,
		Original: elem,
		Nillable: elem.Nillable,
		Abstract: elem.Abstract,
		Default:  elem.Default,
		Fixed:    elem.Fixed,
		HasFixed: elem.HasFixed,
		Block:    elem.Block,
	}
	compiled.EffectiveQName = c.effectiveElementQName(compiled)

	// link to compiled type
	// per XSD spec, if element is in a substitution group and has no explicit type,
	// it inherits the type from the head element
	typeToCompile := elem.Type
	if typeToCompile != nil && c.isDefaultAnyType(typeToCompile) && !elem.SubstitutionGroup.IsZero() {
		// check if we can inherit from substitution group head
		if headDecl, ok := c.schema.ElementDecls[elem.SubstitutionGroup]; ok && headDecl.Type != nil {
			typeToCompile = headDecl.Type
		}
	}

	if typeToCompile != nil {
		typeCompiled, err := c.compileType(typeToCompile.Name(), typeToCompile)
		if err != nil {
			return nil, err
		}
		compiled.Type = typeCompiled
	}

	if len(elem.Constraints) > 0 {
		compiled.Constraints = make([]*grammar.CompiledConstraint, len(elem.Constraints))
		for i, constraint := range elem.Constraints {
			compiled.Constraints[i] = &grammar.CompiledConstraint{
				Original: constraint,
			}
		}
	}

	// only add top-level elements to the global map/cache.
	if isTopLevel {
		c.elements[qname] = compiled
		c.grammar.Elements[qname] = compiled
	}

	return compiled, nil
}

// isDefaultAnyType checks if a type is the default anyType (assigned by parser when no explicit type)
func (c *Compiler) isDefaultAnyType(typ types.Type) bool {
	if ct, ok := typ.(*types.ComplexType); ok {
		// check if it's the anonymous anyType created by makeAnyType()
		return ct.QName.Local == "anyType" && ct.QName.Namespace == "http://www.w3.org/2001/XMLSchema"
	}
	return false
}

func (c *Compiler) compileTopLevelAttribute(qname types.QName, attr *types.AttributeDecl) (*grammar.CompiledAttribute, error) {
	// check if already compiled
	if compiled, ok := c.grammar.Attributes[qname]; ok {
		return compiled, nil
	}

	compiled := &grammar.CompiledAttribute{
		QName:    qname,
		Original: attr,
		Use:      attr.Use,
		Default:  attr.Default,
		Fixed:    attr.Fixed,
		HasFixed: attr.HasFixed,
	}

	if attr.Type != nil {
		attrTypeCompiled, err := c.compileType(attr.Type.Name(), attr.Type)
		if err != nil {
			return nil, err
		}
		compiled.Type = attrTypeCompiled
	}

	c.grammar.Attributes[qname] = compiled

	return compiled, nil
}

func (c *Compiler) computeSubstitutionGroups() {
	for _, elem := range c.grammar.Elements {
		if !elem.Original.SubstitutionGroup.IsZero() {
			head := elem.Original.SubstitutionGroup
			if headElem, ok := c.grammar.Elements[head]; ok {
				elem.SubstitutionHead = headElem

				// add to head's substitutes
				if c.grammar.SubstitutionGroups[head] == nil {
					c.grammar.SubstitutionGroups[head] = make([]*grammar.CompiledElement, 0)
				}
				c.grammar.SubstitutionGroups[head] = append(c.grammar.SubstitutionGroups[head], elem)
			}
		}
	}

	for head, subs := range c.grammar.SubstitutionGroups {
		c.grammar.SubstitutionGroups[head] = c.expandSubstitutes(subs, make(map[types.QName]bool))
	}
}

func (c *Compiler) expandSubstitutes(subs []*grammar.CompiledElement, visited map[types.QName]bool) []*grammar.CompiledElement {
	var result []*grammar.CompiledElement
	for _, elem := range subs {
		if visited[elem.QName] {
			continue
		}
		visited[elem.QName] = true
		result = append(result, elem)

		// recursively add substitutes of this element
		if transSubs, ok := c.grammar.SubstitutionGroups[elem.QName]; ok {
			result = append(result, c.expandSubstitutes(transSubs, visited)...)
		}
	}
	return result
}

// Helper methods
func (c *Compiler) getGroupKind(particle types.Particle) types.GroupKind {
	if mg, ok := particle.(*types.ModelGroup); ok {
		return mg.Kind
	}
	return types.Sequence
}

func getIDTypeName(typeName string) string {
	switch typeName {
	case "ID", "IDREF", "IDREFS":
		return typeName
	default:
		return ""
	}
}

// collectElementsWithConstraints precomputes all elements with identity constraints.
// This includes top-level elements and local elements found in content models.
func (c *Compiler) collectElementsWithConstraints() {
	seen := make(map[*grammar.CompiledElement]bool)
	visitedTypes := make(map[*grammar.CompiledType]bool)

	for _, elem := range c.grammar.Elements {
		if len(elem.Constraints) > 0 && !seen[elem] {
			c.grammar.ElementsWithConstraints = append(c.grammar.ElementsWithConstraints, elem)
			seen[elem] = true
		}
		// also traverse element's type for local elements with constraints
		c.collectConstraintElementsFromType(elem.Type, seen, visitedTypes)
	}

	// collect from all named types (for types not reachable from elements)
	for _, ct := range c.grammar.Types {
		c.collectConstraintElementsFromType(ct, seen, visitedTypes)
	}

	// also check anonymous types
	for _, ct := range c.anonymousTypes {
		c.collectConstraintElementsFromType(ct, seen, visitedTypes)
	}
}

// collectConstraintElementsFromType traverses a type's content model to find elements with constraints.
func (c *Compiler) collectConstraintElementsFromType(ct *grammar.CompiledType, seen map[*grammar.CompiledElement]bool, visitedTypes map[*grammar.CompiledType]bool) {
	if ct == nil || visitedTypes[ct] {
		return
	}
	visitedTypes[ct] = true

	if ct.ContentModel == nil {
		return
	}

	for _, elem := range ct.ContentModel.AllElements {
		if elem != nil && elem.Element != nil {
			if len(elem.Element.Constraints) > 0 && !seen[elem.Element] {
				c.grammar.ElementsWithConstraints = append(c.grammar.ElementsWithConstraints, elem.Element)
				seen[elem.Element] = true
			}
			c.collectConstraintElementsFromType(elem.Element.Type, seen, visitedTypes)
		}
	}

	c.collectConstraintElementsFromParticles(ct.ContentModel.Particles, seen, visitedTypes)
}

// collectConstraintElementsFromParticles traverses particles to find elements with constraints.
func (c *Compiler) collectConstraintElementsFromParticles(particles []*grammar.CompiledParticle, seen map[*grammar.CompiledElement]bool, visitedTypes map[*grammar.CompiledType]bool) {
	for _, p := range particles {
		switch p.Kind {
		case grammar.ParticleElement:
			if p.Element != nil {
				if len(p.Element.Constraints) > 0 && !seen[p.Element] {
					c.grammar.ElementsWithConstraints = append(c.grammar.ElementsWithConstraints, p.Element)
					seen[p.Element] = true
				}
				c.collectConstraintElementsFromType(p.Element.Type, seen, visitedTypes)
			}
		case grammar.ParticleGroup:
			c.collectConstraintElementsFromParticles(p.Children, seen, visitedTypes)
		}
	}
}

// indexLocalElements precomputes the index of all local (non-top-level) elements.
// This is used for XPath evaluation in identity constraints.
func (c *Compiler) indexLocalElements() {
	visitedTypes := make(map[*grammar.CompiledType]bool)

	for _, ct := range c.grammar.Types {
		if ct.ContentModel != nil {
			c.indexLocalElementsFromParticles(ct.ContentModel.Particles, visitedTypes)
		}
	}

	// index local elements from all top-level element types (for anonymous types)
	for _, elem := range c.grammar.Elements {
		if elem.Type != nil && elem.Type.ContentModel != nil {
			if !visitedTypes[elem.Type] {
				visitedTypes[elem.Type] = true
				c.indexLocalElementsFromParticles(elem.Type.ContentModel.Particles, visitedTypes)
			}
		}
	}

	for _, ct := range c.anonymousTypes {
		if ct.ContentModel != nil && !visitedTypes[ct] {
			visitedTypes[ct] = true
			c.indexLocalElementsFromParticles(ct.ContentModel.Particles, visitedTypes)
		}
	}
}

// indexLocalElementsFromParticles recursively indexes local elements from content model particles.
func (c *Compiler) indexLocalElementsFromParticles(particles []*grammar.CompiledParticle, visitedTypes map[*grammar.CompiledType]bool) {
	for _, p := range particles {
		switch p.Kind {
		case grammar.ParticleElement:
			if p.Element != nil {
				// check if this element is not already a top-level element
				if _, isTopLevel := c.grammar.Elements[p.Element.QName]; !isTopLevel {
					effectiveQName := p.Element.EffectiveQName
					if effectiveQName.IsZero() {
						effectiveQName = c.effectiveElementQName(p.Element)
					}
					c.grammar.LocalElements[effectiveQName] = p.Element
				}
				// recursively index elements from this element's type
				if p.Element.Type != nil && p.Element.Type.ContentModel != nil {
					if !visitedTypes[p.Element.Type] {
						visitedTypes[p.Element.Type] = true
						c.indexLocalElementsFromParticles(p.Element.Type.ContentModel.Particles, visitedTypes)
					}
				}
			}
		case grammar.ParticleGroup:
			c.indexLocalElementsFromParticles(p.Children, visitedTypes)
		}
	}
}

// effectiveElementQName computes the effective QName for an element based on its form.
func (c *Compiler) effectiveElementQName(elem *grammar.CompiledElement) types.QName {
	if elem.Original == nil {
		return elem.QName
	}

	switch elem.Original.Form {
	case types.FormQualified:
		return elem.QName
	case types.FormUnqualified:
		return types.QName{Namespace: "", Local: elem.QName.Local}
	default: // FormDefault - use schema's elementFormDefault
		if c.grammar.ElementFormDefault == xsdschema.Qualified {
			return elem.QName
		}
		return types.QName{Namespace: "", Local: elem.QName.Local}
	}
}
