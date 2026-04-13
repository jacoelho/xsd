package semantics

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type typeDefinitionContext int

const (
	typeDefinitionGlobal typeDefinitionContext = iota
	typeDefinitionInline
)

// validateContentStructure validates structural constraints of content.
// Does not validate references, which may be forward references or imports.
func validateContentStructure(schema *parser.Schema, content model.Content, context typeDefinitionContext) error {
	switch c := content.(type) {
	case *model.ElementContent:
		if err := validateParticleStructure(schema, c.Particle); err != nil {
			return err
		}
		if err := validateElementDeclarationsConsistentInParticle(schema, c.Particle); err != nil {
			return err
		}
	case *model.SimpleContent:
		return validateSimpleContentStructure(schema, c, context)
	case *model.ComplexContent:
		return validateComplexContentStructure(schema, c)
	case *model.EmptyContent:
	}
	return nil
}

// validateParticleReferences validates references within particles.
func validateParticleReferences(schema *parser.Schema, particle model.Particle, originLocation string) error {
	visited := make(map[*model.ModelGroup]bool)
	return validateParticleReferencesWithVisited(schema, particle, visited, originLocation)
}

// validateParticleReferencesWithVisited validates references with cycle detection.
func validateParticleReferencesWithVisited(schema *parser.Schema, particle model.Particle, visited map[*model.ModelGroup]bool, originLocation string) error {
	switch p := particle.(type) {
	case *model.ModelGroup:
		return validateModelGroupParticleReferences(schema, p, visited, originLocation)
	case *model.ElementDecl:
		return validateElementDeclParticleReferences(schema, p, visited, originLocation)
	case *model.AnyElement:
		// Wildcards do not have references.
	}
	return nil
}

func validateModelGroupParticleReferences(schema *parser.Schema, group *model.ModelGroup, visited map[*model.ModelGroup]bool, originLocation string) error {
	if visited[group] {
		return nil
	}
	visited[group] = true

	for _, childParticle := range group.Particles {
		if err := validateParticleReferencesWithVisited(schema, childParticle, visited, originLocation); err != nil {
			return err
		}
	}
	return nil
}

func validateElementDeclParticleReferences(schema *parser.Schema, elem *model.ElementDecl, visited map[*model.ModelGroup]bool, originLocation string) error {
	if elem.IsReference {
		if err := validateImportForNamespaceAtLocation(schema, originLocation, elem.Name.Namespace); err != nil {
			return fmt.Errorf("element reference %s: %w", elem.Name, err)
		}
		return nil
	}
	if elem.Type == nil {
		return nil
	}

	contextNS := elementDeclContextNamespace(elem)
	if err := validateTypeReferenceFromTypeWithVisited(schema, elem.Type, visited, contextNS, originLocation); err != nil {
		return fmt.Errorf("element %s: %w", elem.Name, err)
	}
	if err := validateAttributeValueConstraintsForType(schema, elem.Type); err != nil {
		return fmt.Errorf("element %s: %w", elem.Name, err)
	}
	return nil
}

func elementDeclContextNamespace(elem *model.ElementDecl) model.NamespaceURI {
	if elem.SourceNamespace != "" {
		return elem.SourceNamespace
	}
	return elem.Name.Namespace
}

// validateGroupReferences validates references within group definitions.
func validateGroupReferences(schema *parser.Schema, qname model.QName, group *model.ModelGroup) error {
	visited := make(map[*model.ModelGroup]bool)
	origin := schema.GroupOrigins[qname]
	for _, particle := range group.Particles {
		if err := validateParticleReferencesWithVisited(schema, particle, visited, origin); err != nil {
			return err
		}
	}
	return nil
}

// validateComplexContentStructure validates structural constraints of complex content.
func validateComplexContentStructure(schema *parser.Schema, cc *model.ComplexContent) error {
	if cc.Extension != nil {
		if err := validateComplexContentExtension(schema, cc.Extension); err != nil {
			return err
		}
	}
	if cc.Restriction != nil {
		if err := validateComplexContentRestriction(schema, cc.Restriction); err != nil {
			return err
		}
	}
	return nil
}

func validateComplexContentExtension(schema *parser.Schema, ext *model.Extension) error {
	baseType, baseOK := LookupType(schema, ext.Base)
	if baseOK {
		if _, isSimple := baseType.(*model.SimpleType); isSimple {
			return fmt.Errorf("complexContent extension cannot derive from simpleType '%s'", ext.Base)
		}
	}
	if ext.Particle == nil {
		return nil
	}
	baseParticle, err := validateComplexExtensionBase(schema, ext.Base, baseType)
	if err != nil {
		return err
	}
	if extensionContainsAll(ext.Particle) && !baseParticleIsEmptiable(schema, baseType, baseOK, baseParticle) {
		return fmt.Errorf("xs:all cannot be used in complex content extensions unless base content is emptiable (XSD 1.0 Errata E1-21)")
	}
	return validateComplexContentParticle(schema, ext.Particle)
}

func validateComplexExtensionBase(schema *parser.Schema, baseQName model.QName, baseType model.Type) (model.Particle, error) {
	baseCT, ok := baseType.(*model.ComplexType)
	if !ok {
		return nil, nil
	}
	if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
		return nil, fmt.Errorf("cannot extend simpleContent type '%s' with particles", baseQName)
	}
	baseParticle := EffectiveContentParticle(schema, baseCT)
	if baseMG, ok := baseParticle.(*model.ModelGroup); ok && baseMG.Kind == model.AllGroup && !isEmptiableParticle(baseMG) {
		return nil, fmt.Errorf("cannot extend type with non-emptiable xs:all content model (XSD 1.0)")
	}
	return baseParticle, nil
}

func extensionContainsAll(particle model.Particle) bool {
	mg, ok := particle.(*model.ModelGroup)
	if !ok {
		return false
	}
	if mg.Kind == model.AllGroup {
		return true
	}
	if mg.Kind != model.Sequence && mg.Kind != model.Choice {
		return false
	}
	for _, p := range mg.Particles {
		pmg, ok := p.(*model.ModelGroup)
		if ok && pmg.Kind == model.AllGroup {
			return true
		}
	}
	return false
}

func baseParticleIsEmptiable(schema *parser.Schema, baseType model.Type, baseOK bool, baseParticle model.Particle) bool {
	if !baseOK {
		return false
	}
	if baseParticle != nil {
		return isEmptiableParticle(baseParticle)
	}
	baseCT, ok := baseType.(*model.ComplexType)
	if !ok {
		return false
	}
	baseParticle = EffectiveContentParticle(schema, baseCT)
	if baseParticle == nil {
		return true
	}
	return isEmptiableParticle(baseParticle)
}

func validateComplexContentRestriction(schema *parser.Schema, restriction *model.Restriction) error {
	baseType, baseOK := LookupType(schema, restriction.Base)
	if err := validateComplexRestrictionBase(restriction.Base, baseType, baseOK); err != nil {
		return err
	}
	if err := validateComplexRestrictionParticle(schema, baseType, baseOK, restriction.Particle); err != nil {
		return err
	}
	if baseCT, ok := baseType.(*model.ComplexType); ok {
		return validateComplexRestrictionAttributes(schema, baseCT, restriction)
	}
	return nil
}

func validateComplexRestrictionBase(baseQName model.QName, baseType model.Type, baseOK bool) error {
	if !baseOK {
		return nil
	}
	if _, isSimple := baseType.(*model.SimpleType); isSimple {
		return fmt.Errorf("complexContent restriction cannot derive from simpleType '%s'", baseQName)
	}
	if baseCT, ok := baseType.(*model.ComplexType); ok {
		if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
			return fmt.Errorf("complexContent restriction cannot derive from simpleContent type '%s'", baseQName)
		}
	}
	return nil
}

func validateComplexRestrictionParticle(schema *parser.Schema, baseType model.Type, baseOK bool, particle model.Particle) error {
	if particle == nil {
		return nil
	}
	if baseOK {
		if baseParticle := EffectiveContentParticle(schema, baseType); baseParticle != nil {
			if err := validateParticlePairRestriction(schema, baseParticle, particle); err != nil {
				return err
			}
		}
	}
	return validateComplexContentParticle(schema, particle)
}

func validateComplexContentParticle(schema *parser.Schema, particle model.Particle) error {
	if err := validateParticleStructure(schema, particle); err != nil {
		return err
	}
	return validateElementDeclarationsConsistentInParticle(schema, particle)
}

func validateComplexRestrictionAttributes(schema *parser.Schema, baseCT *model.ComplexType, restriction *model.Restriction) error {
	restrictionAttrs := slices.Clone(restriction.Attributes)
	restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, restriction.AttrGroups)...)
	return validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "complexContent restriction")
}

// validateSimpleContentStructure validates structural constraints of simple content.
func validateSimpleContentStructure(schema *parser.Schema, sc *model.SimpleContent, context typeDefinitionContext) error {
	if sc.Restriction != nil {
		baseType, baseOK := LookupType(schema, sc.Restriction.Base)
		if err := validateSimpleContentRestrictionBase(sc.Restriction.Base, baseType, baseOK); err != nil {
			return err
		}
		if err := validateInlineSimpleContentRestriction(context, sc.Restriction, baseType, baseOK); err != nil {
			return err
		}
		if err := validateSimpleContentRestrictionComplexBase(sc.Restriction.Base, baseType); err != nil {
			return err
		}
		if err := validateSimpleContentRestrictionSimpleType(schema, sc.Restriction); err != nil {
			return err
		}
		if err := validateSimpleContentRestrictionFacets(schema, sc.Restriction); err != nil {
			return err
		}
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			if err := validateSimpleContentRestrictionAttributes(schema, baseCT, sc.Restriction); err != nil {
				return err
			}
		}
	}
	if sc.Extension != nil {
		baseType, _ := LookupType(schema, sc.Extension.Base)
		if baseCT, ok := baseType.(*model.ComplexType); ok {
			if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); !isSimpleContent {
				return fmt.Errorf("simpleContent extension cannot derive from complexType '%s' which does not have simpleContent", sc.Extension.Base)
			}
		}
		if sc.Extension.Base.Namespace == model.XSDNamespace && sc.Extension.Base.Local == string(model.TypeNameAnyType) {
			return fmt.Errorf("simpleContent extension cannot have base type anyType")
		}
	}
	return nil
}

func validateSimpleContentRestrictionBase(baseQName model.QName, baseType model.Type, baseOK bool) error {
	if baseOK {
		if _, isSimpleType := baseType.(*model.SimpleType); isSimpleType {
			return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", baseQName)
		}
		return nil
	}
	if baseQName.Namespace == model.XSDNamespace && model.GetBuiltin(model.TypeName(baseQName.Local)) != nil {
		return fmt.Errorf("simpleContent restriction cannot have simpleType base '%s'", baseQName)
	}
	return nil
}

func validateInlineSimpleContentRestriction(context typeDefinitionContext, restriction *model.Restriction, baseType model.Type, baseOK bool) error {
	if context != typeDefinitionInline || len(restriction.Facets) > 0 {
		return nil
	}
	if !baseOK || baseType == nil {
		if restriction.Base.Namespace == model.XSDNamespace {
			return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", restriction.Base)
		}
		return nil
	}
	if _, isSimpleType := baseType.(*model.SimpleType); isSimpleType {
		return fmt.Errorf("simpleContent restriction in inline complexType cannot restrict simpleType '%s' without facets", restriction.Base)
	}
	return nil
}

func validateSimpleContentRestrictionComplexBase(baseQName model.QName, baseType model.Type) error {
	baseCT, ok := baseType.(*model.ComplexType)
	if !ok || baseCT.QName.Local == "anyType" {
		return nil
	}
	if _, isSimpleContent := baseCT.Content().(*model.SimpleContent); isSimpleContent {
		return nil
	}
	return fmt.Errorf("simpleContent restriction cannot derive from complexType '%s' which does not have simpleContent", baseQName)
}

func validateSimpleContentRestrictionSimpleType(schema *parser.Schema, restriction *model.Restriction) error {
	if restriction.SimpleType == nil {
		return nil
	}
	baseSimpleType, baseQName := resolveSimpleContentBaseTypeQName(schema, restriction.Base)
	if baseSimpleType == nil {
		return nil
	}
	if restriction.SimpleType.List != nil || restriction.SimpleType.Union != nil {
		if baseQName.Namespace != model.XSDNamespace || baseQName.Local != string(model.TypeNameAnySimpleType) {
			return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
		}
		return nil
	}
	if restriction.SimpleType.Restriction == nil {
		return nil
	}
	nestedBase := resolveSimpleTypeRestrictionBase(schema, restriction.SimpleType, restriction.SimpleType.Restriction)
	if nestedBase != nil && !model.IsValidlyDerivedFrom(nestedBase, baseSimpleType) {
		return fmt.Errorf("simpleContent restriction simpleType is not derived from base type '%s'", baseQName)
	}
	return nil
}

func validateSimpleContentRestrictionAttributes(schema *parser.Schema, baseCT *model.ComplexType, restriction *model.Restriction) error {
	restrictionAttrs := slices.Clone(restriction.Attributes)
	restrictionAttrs = append(restrictionAttrs, collectAttributesFromGroups(schema, restriction.AttrGroups)...)
	return validateRestrictionAttributes(schema, baseCT, restrictionAttrs, "simpleContent restriction")
}

func validateRestrictionAttributes(schema *parser.Schema, baseCT *model.ComplexType, restrictionAttrs []*model.AttributeDecl, context string) error {
	if baseCT == nil {
		return nil
	}
	baseAttrMap := collectEffectiveAttributeUses(schema, baseCT)
	baseAnyAttr, err := collectAnyAttributeFromType(schema, baseCT)
	if err != nil {
		return err
	}
	for _, restrictionAttr := range restrictionAttrs {
		effectiveRestriction := effectiveAttributeUse(schema, restrictionAttr)
		key := parser.EffectiveAttributeQName(schema, effectiveRestriction)
		baseAttr, exists := baseAttrMap[key]
		if !exists {
			if baseAnyAttr == nil || !model.AllowsNamespace(
				baseAnyAttr.Namespace,
				baseAnyAttr.NamespaceList,
				baseAnyAttr.TargetNamespace,
				key.Namespace,
			) {
				return fmt.Errorf("%s: attribute '%s' not present in base type", context, restrictionAttr.Name.Local)
			}
			continue
		}
		effectiveBase := effectiveAttributeUse(schema, baseAttr)
		if effectiveBase.Use == model.Required && effectiveRestriction.Use != model.Required {
			return fmt.Errorf("%s: required attribute '%s' cannot be relaxed", context, restrictionAttr.Name.Local)
		}
		if effectiveRestriction.Use == model.Prohibited {
			continue
		}
		if effectiveBase.HasFixed {
			if !effectiveRestriction.HasFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
			baseType := parser.ResolveTypeReferenceAllowMissing(schema, effectiveBase.Type)
			if baseType == nil {
				baseType = effectiveBase.Type
			}
			baseFixed := normalizeFixedValue(effectiveBase.Fixed, baseType)
			restrFixed := normalizeFixedValue(effectiveRestriction.Fixed, baseType)
			if baseFixed != restrFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, restrictionAttr.Name.Local)
			}
		}
		baseTypeQName := model.QName{}
		if effectiveBase.Type != nil {
			baseTypeQName = effectiveBase.Type.Name()
		}
		restrictionTypeQName := model.QName{}
		if effectiveRestriction.Type != nil {
			restrictionTypeQName = effectiveRestriction.Type.Name()
		}
		if baseTypeQName.IsZero() || restrictionTypeQName.IsZero() {
			continue
		}
		if baseTypeQName != restrictionTypeQName {
			baseType := parser.ResolveTypeReferenceAllowMissing(schema, effectiveBase.Type)
			restrictionType := parser.ResolveTypeReferenceAllowMissing(schema, effectiveRestriction.Type)
			if baseType == nil {
				baseType = effectiveBase.Type
			}
			if restrictionType == nil {
				restrictionType = effectiveRestriction.Type
			}
			if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
				return fmt.Errorf("%s: attribute '%s' type cannot be changed from '%s' to '%s' in restriction (only use can differ)", context, restrictionAttr.Name.Local, baseTypeQName, restrictionTypeQName)
			}
		}
	}
	return nil
}

func effectiveAttributeUse(schema *parser.Schema, attr *model.AttributeDecl) *model.AttributeDecl {
	if attr == nil || !attr.IsReference {
		return attr
	}
	target, ok := schema.AttributeDecls[attr.Name]
	if !ok {
		return attr
	}
	merged := *attr
	if merged.Type == nil {
		merged.Type = target.Type
	}
	if !merged.HasFixed && target.HasFixed {
		merged.Fixed = target.Fixed
		merged.HasFixed = true
	}
	if !merged.HasDefault && target.HasDefault {
		merged.Default = target.Default
		merged.HasDefault = true
	}
	return &merged
}
