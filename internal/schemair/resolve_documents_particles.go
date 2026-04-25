package schemair

import (
	"fmt"

	ast "github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/value"
)

func (r *docResolver) validateComplexPlan(plan ComplexTypePlan) error {
	if err := r.validateElementDeclarationsConsistent(plan.Particle); err != nil {
		return err
	}
	if err := r.validateUniqueParticleAttribution(plan.Particle); err != nil {
		return err
	}
	if err := r.validateAttributeUsesUnique(plan.Attrs); err != nil {
		return err
	}
	idAttrs := 0
	for _, attrUseID := range plan.Attrs {
		if attrUseID == 0 || int(attrUseID) > len(r.out.AttributeUses) {
			continue
		}
		use := r.out.AttributeUses[attrUseID-1]
		if use.Use == AttributeProhibited {
			continue
		}
		isID, err := r.isIDType(use.TypeDecl, make(map[TypeRef]bool))
		if err != nil {
			return err
		}
		if isID {
			idAttrs++
		}
	}
	if idAttrs > 1 {
		return fmt.Errorf("schema ir: complex type %d has multiple ID attributes", plan.TypeDecl)
	}
	return nil
}

func complexAttributeRestrictionContext(decl *ast.ComplexTypeDecl) string {
	if decl != nil && decl.Content == ast.ComplexContentSimple {
		return "simpleContent restriction"
	}
	return "complexContent restriction"
}

func (r *docResolver) validateAttributeUsesUnique(ids []AttributeUseID) error {
	seen := make(map[Name]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 || int(id) > len(r.out.AttributeUses) {
			continue
		}
		use := r.out.AttributeUses[id-1]
		if use.Use == AttributeProhibited {
			continue
		}
		if _, ok := seen[use.Name]; ok {
			return fmt.Errorf("schema ir: attributes: duplicate attribute '%s' in namespace '%s'", use.Name.Local, use.Name.Namespace)
		}
		seen[use.Name] = struct{}{}
	}
	return nil
}

func (r *docResolver) validateAttributeRestriction(baseIDs, restrictionIDs []AttributeUseID, baseAnyAttr WildcardID, context string) error {
	baseByName := make(map[Name]AttributeUse, len(baseIDs))
	for _, id := range baseIDs {
		if id == 0 || int(id) > len(r.out.AttributeUses) {
			continue
		}
		use := r.out.AttributeUses[id-1]
		if use.Use == AttributeProhibited {
			continue
		}
		baseByName[use.Name] = use
	}
	for _, id := range restrictionIDs {
		if id == 0 || int(id) > len(r.out.AttributeUses) {
			continue
		}
		use := r.out.AttributeUses[id-1]
		base, ok := baseByName[use.Name]
		if !ok {
			if !r.wildcardAllowsAttribute(baseAnyAttr, use.Name) {
				return fmt.Errorf("%s: attribute '%s' not present in base type", context, use.Name.Local)
			}
			continue
		}
		if base.Use == AttributeRequired && use.Use != AttributeRequired {
			return fmt.Errorf("%s: required attribute '%s' cannot be relaxed", context, use.Name.Local)
		}
		if use.Use == AttributeProhibited {
			continue
		}
		if base.Fixed.Present {
			baseFixed := r.normalizeValueConstraintLexical(base.TypeDecl, base.Fixed)
			useFixed := r.normalizeValueConstraintLexical(base.TypeDecl, use.Fixed)
			if !use.Fixed.Present || useFixed != baseFixed {
				return fmt.Errorf("%s: attribute '%s' fixed value must match base type", context, use.Name.Local)
			}
		}
		if err := r.validateAttributeTypeRestriction(base, use, context); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) validateAttributeTypeRestriction(base, restriction AttributeUse, context string) error {
	baseType := base.TypeDecl
	restrictionType := restriction.TypeDecl
	if isZeroTypeRef(baseType) || isZeroTypeRef(restrictionType) {
		return nil
	}
	if sameTypeRef(baseType, restrictionType) {
		return nil
	}
	if ok, err := r.typeRestrictsUnionMember(restrictionType, baseType); err != nil {
		return err
	} else if ok {
		return nil
	}
	if mask, ok, err := r.derivationMask(restrictionType, baseType); err != nil {
		return err
	} else if ok {
		if mask&DerivationExtension != 0 {
			return fmt.Errorf("%s: attribute '%s' type cannot be changed from '%s' to '%s' in restriction (only use can differ)",
				context, restriction.Name.Local, formatName(baseType.Name), formatName(restrictionType.Name))
		}
		return nil
	}
	return fmt.Errorf("%s: attribute '%s' type cannot be changed from '%s' to '%s' in restriction (only use can differ)",
		context, restriction.Name.Local, formatName(baseType.Name), formatName(restrictionType.Name))
}

func (r *docResolver) wildcardAllowsAttribute(id WildcardID, name Name) bool {
	if id == 0 || int(id) > len(r.out.Wildcards) {
		return false
	}
	wildcard := r.out.Wildcards[id-1]
	namespaces := make([]ast.NamespaceURI, 0, len(wildcard.Namespaces))
	for _, namespace := range wildcard.Namespaces {
		namespaces = append(namespaces, ast.NamespaceURI(namespace))
	}
	return ast.AllowsNamespace(
		astNamespaceKind(wildcard.NamespaceKind),
		namespaces,
		ast.NamespaceURI(wildcard.TargetNamespace),
		ast.NamespaceURI(name.Namespace),
	)
}

func (r *docResolver) validateAnyAttributeRestriction(base, derived WildcardID) error {
	if err := r.validateWildcardToWildcardRestriction(base, derived); err != nil {
		return fmt.Errorf("schema ir: anyAttribute derivation: anyAttribute restriction: derived anyAttribute is not a valid subset of base anyAttribute")
	}
	return nil
}

func (r *docResolver) normalizeValueConstraintLexical(ref TypeRef, constraint ValueConstraint) string {
	if !constraint.Present {
		return ""
	}
	spec, ok := r.specForRef(ref)
	if !ok {
		return constraint.Lexical
	}
	normalized := value.NormalizeWhitespace(valueWhitespaceMode(spec.Whitespace), []byte(constraint.Lexical), nil)
	return string(normalized)
}

func valueWhitespaceMode(mode WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case WhitespaceReplace:
		return value.WhitespaceReplace
	case WhitespaceCollapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}

func (r *docResolver) validateElementDeclarationsConsistent(particleID ParticleID) error {
	seen := make(map[Name]TypeRef)
	visiting := make(map[ParticleID]bool)
	return r.validateElementDeclarationsConsistentParticle(particleID, seen, visiting)
}

func (r *docResolver) validateElementDeclarationsConsistentParticle(
	particleID ParticleID,
	seen map[Name]TypeRef,
	visiting map[ParticleID]bool,
) error {
	if particleID == 0 {
		return nil
	}
	if visiting[particleID] {
		return nil
	}
	if int(particleID) > len(r.out.Particles) {
		return fmt.Errorf("schema ir: particle %d not found", particleID)
	}
	visiting[particleID] = true
	particle := r.out.Particles[particleID-1]
	switch particle.Kind {
	case ParticleElement:
		elem, ok := r.emittedElement(particle.Element)
		if !ok {
			return nil
		}
		if existing, ok := seen[elem.Name]; ok {
			if !sameTypeRef(existing, elem.TypeDecl) {
				return fmt.Errorf("schema ir: duplicate local element declaration '%s' with different types", elem.Name.Local)
			}
			return nil
		}
		seen[elem.Name] = elem.TypeDecl
	case ParticleGroup:
		for _, childID := range particle.Children {
			if err := r.validateElementDeclarationsConsistentParticle(childID, seen, visiting); err != nil {
				return err
			}
		}
	}
	delete(visiting, particleID)
	return nil
}

type upaTerm struct {
	element            ElementID
	wildcard           WildcardID
	allowsSubstitution bool
}

func (r *docResolver) validateUniqueParticleAttribution(particleID ParticleID) error {
	if particleID == 0 {
		return nil
	}
	particle, ok, err := r.particle(particleID)
	if err != nil || !ok {
		return err
	}
	if particle.Kind != ParticleGroup {
		return nil
	}
	switch particle.Group {
	case GroupChoice, GroupAll:
		if err := r.validateChoiceUPA(particle.Children); err != nil {
			return err
		}
	case GroupSequence:
		if err := r.validateSequenceUPA(particle.Children); err != nil {
			return err
		}
	}
	for _, child := range particle.Children {
		if err := r.validateUniqueParticleAttribution(child); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) validateChoiceUPA(children []ParticleID) error {
	for i := range children {
		left, err := r.firstUPATerms(children[i])
		if err != nil {
			return err
		}
		for j := i + 1; j < len(children); j++ {
			right, err := r.firstUPATerms(children[j])
			if err != nil {
				return err
			}
			if r.upaTermsOverlap(left, right) {
				return fmt.Errorf("schema ir: content model is not deterministic: particles overlap")
			}
		}
	}
	return nil
}

func (r *docResolver) validateSequenceUPA(children []ParticleID) error {
	for i := range children {
		left, err := r.sequenceCompetingUPATerms(children[i])
		if err != nil {
			return err
		}
		if len(left) == 0 {
			continue
		}
		for j := i + 1; j < len(children); j++ {
			right, err := r.firstUPATerms(children[j])
			if err != nil {
				return err
			}
			if r.upaTermsOverlap(left, right) {
				return fmt.Errorf("schema ir: content model is not deterministic: particles overlap")
			}
			if !r.particleNullable(children[j]) {
				break
			}
		}
	}
	return nil
}

func (r *docResolver) sequenceCompetingUPATerms(particleID ParticleID) ([]upaTerm, error) {
	var out []upaTerm
	if r.particleCanCompeteWithFollowing(particleID) {
		terms, err := r.firstUPATerms(particleID)
		if err != nil {
			return nil, err
		}
		out = append(out, terms...)
	}
	trailing, err := r.trailingCompetingUPATerms(particleID)
	if err != nil {
		return nil, err
	}
	return append(out, trailing...), nil
}

func (r *docResolver) trailingCompetingUPATerms(particleID ParticleID) ([]upaTerm, error) {
	particle, ok, err := r.particle(particleID)
	if err != nil || !ok {
		return nil, err
	}
	if particleIsExcluded(particle) {
		return nil, nil
	}
	if particle.Kind != ParticleGroup {
		if !r.particleCanCompeteWithFollowing(particleID) {
			return nil, nil
		}
		return r.firstUPATerms(particleID)
	}

	var out []upaTerm
	if r.particleCanCompeteWithFollowing(particleID) {
		terms, err := r.firstUPATerms(particleID)
		if err != nil {
			return nil, err
		}
		out = append(out, terms...)
	}

	switch particle.Group {
	case GroupSequence:
		for i := len(particle.Children) - 1; i >= 0; i-- {
			terms, err := r.trailingCompetingUPATerms(particle.Children[i])
			if err != nil {
				return nil, err
			}
			out = append(out, terms...)
			if !r.particleNullable(particle.Children[i]) {
				break
			}
		}
	default:
		for _, child := range particle.Children {
			terms, err := r.trailingCompetingUPATerms(child)
			if err != nil {
				return nil, err
			}
			out = append(out, terms...)
		}
	}
	return out, nil
}

func (r *docResolver) firstUPATerms(particleID ParticleID) ([]upaTerm, error) {
	particle, ok, err := r.particle(particleID)
	if err != nil || !ok {
		return nil, err
	}
	if particleIsExcluded(particle) {
		return nil, nil
	}
	switch particle.Kind {
	case ParticleElement:
		return []upaTerm{{
			element:            particle.Element,
			allowsSubstitution: particle.AllowsSubstitution,
		}}, nil
	case ParticleWildcard:
		return []upaTerm{{wildcard: particle.Wildcard}}, nil
	case ParticleGroup:
		var out []upaTerm
		switch particle.Group {
		case GroupSequence:
			for _, child := range particle.Children {
				terms, err := r.firstUPATerms(child)
				if err != nil {
					return nil, err
				}
				out = append(out, terms...)
				if !r.particleNullable(child) {
					break
				}
			}
		default:
			for _, child := range particle.Children {
				terms, err := r.firstUPATerms(child)
				if err != nil {
					return nil, err
				}
				out = append(out, terms...)
			}
		}
		return out, nil
	default:
		return nil, nil
	}
}

func (r *docResolver) particleNullable(particleID ParticleID) bool {
	particle, ok, err := r.particle(particleID)
	if err != nil || !ok {
		return false
	}
	min, _, err := r.effectiveParticleOccurrence(particle)
	return err == nil && !min.Unbounded && min.Value == 0
}

func (r *docResolver) particleCanCompeteWithFollowing(particleID ParticleID) bool {
	particle, ok, err := r.particle(particleID)
	if err != nil || !ok {
		return false
	}
	minOcc, maxOcc, err := r.effectiveParticleOccurrence(particle)
	return err == nil && (minOcc.Value == 0 || maxOcc.Unbounded || maxOcc.Value > minOcc.Value)
}

func (r *docResolver) upaTermsOverlap(left, right []upaTerm) bool {
	for _, a := range left {
		for _, b := range right {
			if r.upaTermOverlap(a, b) {
				return true
			}
		}
	}
	return false
}

func (r *docResolver) upaTermOverlap(a, b upaTerm) bool {
	switch {
	case a.element != 0 && b.element != 0:
		return r.upaElementsOverlap(a, b)
	case a.element != 0 && b.wildcard != 0:
		return r.upaElementWildcardOverlap(a, b.wildcard)
	case a.wildcard != 0 && b.element != 0:
		return r.upaElementWildcardOverlap(b, a.wildcard)
	case a.wildcard != 0 && b.wildcard != 0:
		return r.upaWildcardsOverlap(a.wildcard, b.wildcard)
	default:
		return false
	}
}

func (r *docResolver) upaElementsOverlap(a, b upaTerm) bool {
	left, leftOK := r.emittedElement(a.element)
	right, rightOK := r.emittedElement(b.element)
	if !leftOK || !rightOK {
		return false
	}
	if left.Name == right.Name {
		return true
	}
	return (a.allowsSubstitution && r.elementSubstitutesFor(b.element, a.element)) ||
		(b.allowsSubstitution && r.elementSubstitutesFor(a.element, b.element))
}

func (r *docResolver) upaElementWildcardOverlap(elemTerm upaTerm, wildcardID WildcardID) bool {
	elem, ok := r.emittedElement(elemTerm.element)
	if !ok {
		return false
	}
	if r.wildcardAllowsNamespace(wildcardID, elem.Name.Namespace) {
		return true
	}
	if !elemTerm.allowsSubstitution {
		return false
	}
	for _, candidate := range r.out.Elements {
		if !r.elementSubstitutesFor(candidate.ID, elemTerm.element) {
			continue
		}
		if r.wildcardAllowsNamespace(wildcardID, candidate.Name.Namespace) {
			return true
		}
	}
	return false
}

func (r *docResolver) upaWildcardsOverlap(left, right WildcardID) bool {
	return ast.IntersectAnyElement(r.anyElementFromWildcard(left), r.anyElementFromWildcard(right)) != nil
}

func (r *docResolver) validateParticleRestriction(baseID, restrictionID ParticleID) error {
	base, ok, err := r.particle(baseID)
	if err != nil || !ok {
		return err
	}
	restriction, ok, err := r.particle(restrictionID)
	if err != nil || !ok {
		return err
	}
	return r.validateParticleRestrictionValue(base, restriction)
}

func (r *docResolver) validateParticleRestrictionValue(base Particle, restriction Particle) error {
	var err error
	base, err = r.normalizePointlessIRParticle(base)
	if err != nil {
		return err
	}
	restriction, err = r.normalizePointlessIRParticle(restriction)
	if err != nil {
		return err
	}
	if base.Kind == ParticleGroup && restriction.Kind != ParticleGroup {
		return r.validateParticleAgainstGroupRestriction(base, restriction)
	}
	if base.Kind == ParticleWildcard && restriction.Kind == ParticleGroup {
		return r.validateWildcardToGroupParticleRestriction(base, restriction)
	}
	if base.Kind == ParticleGroup && restriction.Kind == ParticleGroup && base.Group == GroupChoice && base.Group != restriction.Group {
		return r.validateChoiceGroupRestriction(base, restriction)
	}
	if err := validateDocumentOccurrenceRestriction(base.Min, base.Max, restriction.Min, restriction.Max); err != nil {
		return err
	}
	if base.Kind == ParticleWildcard {
		return r.validateWildcardParticleRestriction(base, restriction)
	}
	if base.Kind == ParticleElement && restriction.Kind == ParticleGroup && restriction.Group == GroupChoice {
		ok, err := r.choiceRestrictsElement(base, restriction)
		if err != nil || ok {
			return err
		}
		return fmt.Errorf("ComplexContent restriction: choice is not a valid restriction of element")
	}
	if base.Kind == ParticleGroup && restriction.Kind == ParticleGroup {
		if handled, err := r.validateSingleWildcardGroupRestriction(base, restriction); handled {
			return err
		}
	}
	if base.Kind == ParticleGroup && restriction.Kind == ParticleGroup && base.Group == restriction.Group {
		if base.Group == GroupChoice {
			return r.validateChoiceParticleRestriction(base, restriction)
		}
		if err := r.validateChoiceSubsetRestriction(base, restriction); err != nil {
			return err
		}
		if base.Group == GroupSequence {
			return r.validateSequenceParticleRestriction(base, restriction)
		}
		if base.Group == GroupAll {
			return r.validateSameAllGroupRestriction(base, restriction)
		}
		limit := min(len(base.Children), len(restriction.Children))
		for i := 0; i < limit; i++ {
			if base.Group == GroupChoice {
				child, ok, err := r.particle(restriction.Children[i])
				if err != nil || !ok {
					return err
				}
				if particleIsExcluded(child) {
					continue
				}
			}
			if err := r.validateParticleRestriction(base.Children[i], restriction.Children[i]); err != nil {
				return err
			}
		}
		if base.Group != GroupChoice {
			for i := limit; i < len(base.Children); i++ {
				child, ok, err := r.particle(base.Children[i])
				if err != nil || !ok {
					return err
				}
				if r.particleIsRequired(child) {
					return fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
				}
			}
		}
		return nil
	}
	if base.Kind == ParticleGroup && restriction.Kind == ParticleGroup && base.Group != restriction.Group {
		if handled, err := r.validateGroupKindChangeWithWildcard(base, restriction); handled {
			return err
		}
		if base.Group == GroupSequence && restriction.Group == GroupAll {
			count, err := r.activeGroupChildCount(restriction)
			if err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("ComplexContent restriction: cannot restrict sequence to xs:all")
			}
		}
		if base.Group == GroupSequence && restriction.Group == GroupChoice {
			return fmt.Errorf("ComplexContent restriction: cannot restrict sequence to choice")
		}
		if base.Group == GroupSequence {
			return r.validateSequenceParticleRestriction(base, restriction)
		}
		if base.Group == GroupChoice {
			return r.validateChoiceGroupRestriction(base, restriction)
		}
		if base.Group == GroupAll {
			return r.validateAllGroupBaseRestriction(base, restriction)
		}
		return fmt.Errorf("ComplexContent restriction: model group kind mismatch")
	}
	if base.Kind == ParticleElement && restriction.Kind == ParticleElement {
		return r.validateElementParticleRestriction(base, restriction)
	}
	if base.Kind != restriction.Kind {
		if restriction.Kind == ParticleWildcard {
			return fmt.Errorf("ComplexContent restriction: cannot restrict non-wildcard to wildcard")
		}
		return fmt.Errorf("ComplexContent restriction: particle kind mismatch")
	}
	return nil
}

func (r *docResolver) validateChoiceParticleRestriction(base, restriction Particle) error {
	baseIndex := 0
	for _, restrictionChildID := range restriction.Children {
		child, ok, err := r.particle(restrictionChildID)
		if err != nil || !ok {
			return err
		}
		if particleIsExcluded(child) {
			continue
		}
		if child.Kind == ParticleGroup && len(restriction.Children) < len(base.Children) {
			if err := validateDocumentOccurrenceRestriction(base.Min, base.Max, child.Min, child.Max); err != nil {
				return err
			}
		}
		matched := false
		for baseIndex < len(base.Children) {
			baseChildID := base.Children[baseIndex]
			baseIndex++
			compatible, err := r.particlesCanRestrict(baseChildID, restrictionChildID)
			if err != nil {
				return err
			}
			if !compatible {
				continue
			}
			if err := r.validateParticleRestriction(baseChildID, restrictionChildID); err != nil {
				return err
			}
			matched = true
			break
		}
		if !matched {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle in choice")
		}
	}
	return nil
}

func (r *docResolver) validateSameAllGroupRestriction(base, restriction Particle) error {
	baseMin, baseMax, err := r.effectiveParticleOccurrence(base)
	if err != nil {
		return err
	}
	restrictionMin, restrictionMax, err := r.effectiveParticleOccurrence(restriction)
	if err != nil {
		return err
	}
	if err := validateDocumentOccurrenceRestriction(baseMin, baseMax, restrictionMin, restrictionMax); err != nil {
		return err
	}
	limit := min(len(base.Children), len(restriction.Children))
	for i := 0; i < limit; i++ {
		if err := r.validateParticleRestriction(base.Children[i], restriction.Children[i]); err != nil {
			return err
		}
	}
	for i := limit; i < len(restriction.Children); i++ {
		child, ok, err := r.particle(restriction.Children[i])
		if err != nil || !ok {
			return err
		}
		if !particleIsExcluded(child) {
			return fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
	}
	for i := limit; i < len(base.Children); i++ {
		child, ok, err := r.particle(base.Children[i])
		if err != nil || !ok {
			return err
		}
		if r.particleIsRequired(child) {
			return fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
		}
	}
	return nil
}

func (r *docResolver) validateElementParticleRestriction(base, restriction Particle) error {
	baseElem, ok := r.emittedElement(base.Element)
	if !ok {
		return nil
	}
	restrictionElem, ok := r.emittedElement(restriction.Element)
	if !ok {
		return nil
	}
	if baseElem.Name != restrictionElem.Name && !r.elementSubstitutesFor(restriction.Element, base.Element) {
		return fmt.Errorf("ComplexContent restriction: element name mismatch (%s vs %s)",
			formatName(baseElem.Name), formatName(restrictionElem.Name))
	}
	if !baseElem.Nillable && restrictionElem.Nillable {
		return fmt.Errorf("ComplexContent restriction: element '%s' nillable cannot be true when base element nillable is false",
			formatName(restrictionElem.Name))
	}
	if baseElem.Fixed.Present {
		if !restrictionElem.Fixed.Present {
			return fmt.Errorf("ComplexContent restriction: element '%s' must have fixed value matching base fixed value '%s'",
				formatName(restrictionElem.Name), baseElem.Fixed.Lexical)
		}
		baseFixed := r.normalizeValueConstraintLexical(baseElem.TypeDecl, baseElem.Fixed)
		restrictionFixed := r.normalizeValueConstraintLexical(restrictionElem.TypeDecl, restrictionElem.Fixed)
		if baseFixed != restrictionFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' fixed value '%s' must match base fixed value '%s'",
				formatName(restrictionElem.Name), restrictionElem.Fixed.Lexical, baseElem.Fixed.Lexical)
		}
	}
	if restrictionElem.Block&baseElem.Block != baseElem.Block {
		return fmt.Errorf("ComplexContent restriction: element '%s' block constraint must be superset of base block constraint",
			formatName(restrictionElem.Name))
	}
	return r.validateElementTypeRestriction(baseElem, restrictionElem)
}

func (r *docResolver) validateElementTypeRestriction(baseElem, restrictionElem Element) error {
	baseType := baseElem.TypeDecl
	restrictionType := restrictionElem.TypeDecl
	if isZeroTypeRef(baseType) || isZeroTypeRef(restrictionType) {
		return nil
	}
	if baseType.Builtin && baseType.Name.Local == "anyType" {
		return nil
	}
	if baseType.Builtin && baseType.Name.Local == "anySimpleType" {
		info, ok, err := r.typeInfoForRef(restrictionType)
		if err != nil || !ok {
			return err
		}
		if info.Kind != TypeComplex {
			return nil
		}
	}
	if sameTypeRef(baseType, restrictionType) {
		return nil
	}
	if ok, err := r.typeRestrictsUnionMember(restrictionType, baseType); err != nil {
		return err
	} else if ok {
		return nil
	}
	if mask, ok, err := r.derivationMask(restrictionType, baseType); err != nil {
		return err
	} else if ok {
		if mask&DerivationExtension != 0 {
			return fmt.Errorf("ComplexContent restriction: element '%s' type '%s' must be same as or restriction-derived from base type '%s'",
				formatName(restrictionElem.Name), formatName(restrictionType.Name), formatName(baseType.Name))
		}
		return nil
	}
	return fmt.Errorf("ComplexContent restriction: element '%s' type '%s' must be same as or derived from base type '%s'",
		formatName(restrictionElem.Name), formatName(restrictionType.Name), formatName(baseType.Name))
}

func (r *docResolver) typeRestrictsUnionMember(restriction, base TypeRef) (bool, error) {
	spec, ok, err := r.simpleTypeSpecForRef(base)
	if err != nil {
		return false, err
	}
	if !ok || spec.Variety != TypeVarietyUnion {
		return false, nil
	}
	for _, member := range spec.Members {
		if sameTypeRef(restriction, member) {
			return true, nil
		}
		if _, ok, err := r.derivationMask(restriction, member); err != nil {
			return false, err
		} else if ok {
			return true, nil
		}
	}
	return false, nil
}

func (r *docResolver) simpleTypeSpecForRef(ref TypeRef) (SimpleTypeSpec, bool, error) {
	if ref.Builtin {
		for _, builtin := range r.out.BuiltinTypes {
			if builtin.Name == ref.Name {
				return builtin.Value, true, nil
			}
		}
		return SimpleTypeSpec{}, false, nil
	}
	if decl := r.simpleDecls[r.simpleByID[ref.ID]]; decl != nil {
		if _, err := r.ensureSimpleType(decl, !decl.Name.IsZero()); err != nil {
			return SimpleTypeSpec{}, false, err
		}
	}
	for _, spec := range r.out.SimpleTypes {
		if spec.TypeDecl == ref.ID {
			return spec, true, nil
		}
	}
	return SimpleTypeSpec{}, false, nil
}

func (r *docResolver) validateSingleWildcardGroupRestriction(base, restriction Particle) (bool, error) {
	if len(base.Children) != 1 {
		return false, nil
	}
	child, ok, err := r.particle(base.Children[0])
	if err != nil || !ok {
		return true, err
	}
	if child.Kind != ParticleWildcard {
		return false, nil
	}
	return true, r.validateParticleRestriction(child.ID, restriction.ID)
}

func (r *docResolver) validateGroupKindChangeWithWildcard(base, restriction Particle) (bool, error) {
	for _, childID := range base.Children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return true, err
		}
		if child.Kind != ParticleWildcard {
			continue
		}
		if err := r.validateParticleRestriction(childID, restriction.ID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func (r *docResolver) normalizePointlessIRParticle(p Particle) (Particle, error) {
	for p.Kind == ParticleGroup && occursIsOne(p.Min) && occursIsOne(p.Max) {
		children, err := r.derivationChildIDs(p)
		if err != nil {
			return Particle{}, err
		}
		if len(children) != 1 {
			return p, nil
		}
		child, ok, err := r.particle(children[0])
		if err != nil || !ok {
			return Particle{}, err
		}
		p = child
	}
	return p, nil
}

func (r *docResolver) derivationChildIDs(group Particle) ([]ParticleID, error) {
	children := make([]ParticleID, 0, len(group.Children))
	for _, childID := range group.Children {
		childIDs, err := r.gatherPointlessIRChildIDs(group.Group, childID)
		if err != nil {
			return nil, err
		}
		children = append(children, childIDs...)
	}
	return children, nil
}

func (r *docResolver) gatherPointlessIRChildIDs(parent GroupKind, childID ParticleID) ([]ParticleID, error) {
	child, ok, err := r.particle(childID)
	if err != nil || !ok {
		return nil, err
	}
	if child.Kind != ParticleGroup || !occursIsOne(child.Min) || !occursIsOne(child.Max) {
		return []ParticleID{childID}, nil
	}
	if len(child.Children) == 1 {
		return r.gatherPointlessIRChildIDs(parent, child.Children[0])
	}
	if child.Group == parent {
		var out []ParticleID
		for _, nestedID := range child.Children {
			nested, err := r.gatherPointlessIRChildIDs(parent, nestedID)
			if err != nil {
				return nil, err
			}
			out = append(out, nested...)
		}
		return out, nil
	}
	return []ParticleID{childID}, nil
}

func (r *docResolver) activeGroupChildCount(group Particle) (int, error) {
	var count int
	for _, childID := range group.Children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return 0, err
		}
		if particleIsExcluded(child) {
			continue
		}
		count++
	}
	return count, nil
}

func (r *docResolver) activeParticleChildCount(id ParticleID) (int, error) {
	particle, ok, err := r.particle(id)
	if err != nil || !ok {
		return 0, err
	}
	if particle.Kind != ParticleGroup {
		if particleIsExcluded(particle) {
			return 0, nil
		}
		return 1, nil
	}
	return r.activeGroupChildCount(particle)
}

func (r *docResolver) validateSequenceParticleRestriction(base, restriction Particle) error {
	baseChildren, err := r.derivationChildIDs(base)
	if err != nil {
		return err
	}
	restrictionChildren, err := r.derivationChildIDs(restriction)
	if err != nil {
		return err
	}
	baseIndex := 0
	for _, restrictionChildID := range restrictionChildren {
		matched := false
		for baseIndex < len(baseChildren) {
			baseChildID := baseChildren[baseIndex]
			baseChild, ok, err := r.particle(baseChildID)
			if err != nil || !ok {
				return err
			}
			compatible, err := r.particlesCanRestrict(baseChildID, restrictionChildID)
			if err != nil {
				return err
			}
			if compatible {
				if err := r.validateParticleRestriction(baseChildID, restrictionChildID); err != nil {
					return err
				}
				baseIndex++
				matched = true
				break
			}
			if r.particleIsRequired(baseChild) {
				return fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
			}
			baseIndex++
		}
		if !matched {
			return fmt.Errorf("ComplexContent restriction: restriction particle has no matching base particle")
		}
	}
	for ; baseIndex < len(baseChildren); baseIndex++ {
		child, ok, err := r.particle(baseChildren[baseIndex])
		if err != nil || !ok {
			return err
		}
		if r.particleIsRequired(child) {
			return fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
		}
	}
	return nil
}

func (r *docResolver) validateChoiceGroupRestriction(base, restriction Particle) error {
	minOcc, maxOcc, err := r.groupChildCountOccurrence(restriction)
	if err != nil {
		return err
	}
	if err := validateDocumentOccurrenceRestriction(base.Min, base.Max, minOcc, maxOcc); err != nil {
		return err
	}
	for _, restrictionChildID := range restriction.Children {
		child, ok, err := r.particle(restrictionChildID)
		if err != nil || !ok {
			return err
		}
		if particleIsExcluded(child) {
			continue
		}
		matched := false
		for _, baseChildID := range base.Children {
			compatible, err := r.particlesCanRestrict(baseChildID, restrictionChildID)
			if err != nil {
				return err
			}
			if !compatible {
				continue
			}
			if err := r.validateParticleRestriction(baseChildID, restrictionChildID); err != nil {
				return err
			}
			matched = true
			break
		}
		if !matched {
			return fmt.Errorf("ComplexContent restriction: restriction particle has no matching base particle")
		}
	}
	return nil
}

func (r *docResolver) validateAllGroupBaseRestriction(base, restriction Particle) error {
	baseMin, baseMax, err := r.effectiveParticleOccurrence(base)
	if err != nil {
		return err
	}
	restrictionMin, restrictionMax, err := r.effectiveParticleOccurrence(restriction)
	if err != nil {
		return err
	}
	if err := validateDocumentOccurrenceRestriction(baseMin, baseMax, restrictionMin, restrictionMax); err != nil {
		return err
	}
	matchedBase := make([]bool, len(base.Children))
	for _, restrictionChildID := range restriction.Children {
		child, ok, err := r.particle(restrictionChildID)
		if err != nil || !ok {
			return err
		}
		if particleIsExcluded(child) {
			continue
		}
		matched := false
		for i, baseChildID := range base.Children {
			compatible, err := r.particlesCanRestrict(baseChildID, restrictionChildID)
			if err != nil {
				return err
			}
			if !compatible {
				continue
			}
			if err := r.validateParticleRestriction(baseChildID, restrictionChildID); err != nil {
				return err
			}
			matchedBase[i] = true
			matched = true
			break
		}
		if !matched {
			return fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
	}
	for i, childID := range base.Children {
		if matchedBase[i] {
			continue
		}
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return err
		}
		if r.particleIsRequired(child) {
			return fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
		}
	}
	return nil
}

func (r *docResolver) groupChildCountOccurrence(group Particle) (Occurs, Occurs, error) {
	var count uint32
	for _, childID := range group.Children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return Occurs{}, Occurs{}, err
		}
		if particleIsExcluded(child) {
			continue
		}
		count++
	}
	countOccurs := Occurs{Value: count}
	return multiplyOccurs(group.Min, countOccurs), multiplyOccurs(group.Max, countOccurs), nil
}

func (r *docResolver) particlesCanRestrict(baseID, restrictionID ParticleID) (bool, error) {
	base, ok, err := r.particle(baseID)
	if err != nil || !ok {
		return false, err
	}
	restriction, ok, err := r.particle(restrictionID)
	if err != nil || !ok {
		return false, err
	}
	if base.Kind == ParticleWildcard {
		return true, nil
	}
	if base.Kind == ParticleElement && restriction.Kind == ParticleGroup && restriction.Group == GroupChoice {
		return r.choiceRestrictsElement(base, restriction)
	}
	if base.Kind == ParticleGroup && restriction.Kind != ParticleGroup {
		return r.particleCanRestrictGroup(base, restriction)
	}
	if base.Kind != restriction.Kind {
		return false, nil
	}
	if base.Kind == ParticleGroup && base.Group != restriction.Group && base.Group == GroupChoice {
		return true, nil
	}
	if base.Kind == ParticleElement {
		baseElem, ok := r.emittedElement(base.Element)
		if !ok {
			return false, nil
		}
		restrictionElem, ok := r.emittedElement(restriction.Element)
		if !ok {
			return false, nil
		}
		return baseElem.Name == restrictionElem.Name || r.elementSubstitutesFor(restriction.Element, base.Element), nil
	}
	if base.Kind == ParticleGroup {
		return base.Group == restriction.Group, nil
	}
	return true, nil
}

func (r *docResolver) validateParticleAgainstGroupRestriction(base Particle, restriction Particle) error {
	if base.Group == GroupChoice {
		one := Occurs{Value: 1}
		if err := validateDocumentOccurrenceRestriction(base.Min, base.Max, one, one); err != nil {
			return err
		}
	}
	match, ok, err := r.groupParticleRestrictionMatch(base, restriction)
	if err != nil {
		return err
	}
	if !ok {
		if restriction.Kind == ParticleWildcard {
			return fmt.Errorf("ComplexContent restriction: cannot restrict non-wildcard to wildcard")
		}
		return fmt.Errorf("ComplexContent restriction: restriction particle has no matching base particle")
	}
	child, ok, err := r.particle(match)
	if err != nil || !ok {
		return err
	}
	if base.Group == GroupChoice && !occursIsOne(base.Max) {
		child.Min, child.Max = r.choiceChildOccurrenceRange(base, child)
	}
	return r.validateParticleRestrictionValue(child, restriction)
}

func occursIsOne(o Occurs) bool {
	return !o.Unbounded && o.Value == 1
}

func (r *docResolver) choiceChildOccurrenceRange(base Particle, child Particle) (Occurs, Occurs) {
	minOcc := Occurs{}
	if len(base.Children) == 1 {
		minOcc = multiplyOccurs(base.Min, child.Min)
	}
	return minOcc, multiplyOccurs(base.Max, child.Max)
}

func (r *docResolver) particleCanRestrictGroup(base Particle, restriction Particle) (bool, error) {
	_, ok, err := r.groupParticleRestrictionMatch(base, restriction)
	return ok, err
}

func (r *docResolver) groupParticleRestrictionMatch(base Particle, restriction Particle) (ParticleID, bool, error) {
	if base.Group == GroupChoice {
		for _, childID := range base.Children {
			child, ok, err := r.particle(childID)
			if err != nil || !ok {
				return 0, false, err
			}
			if particleIsExcluded(child) {
				continue
			}
			compatible, err := r.particlesCanRestrict(childID, restriction.ID)
			if err != nil || compatible {
				return childID, compatible, err
			}
		}
		return 0, false, nil
	}

	var match ParticleID
	for _, childID := range base.Children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return 0, false, err
		}
		compatible, err := r.particlesCanRestrict(childID, restriction.ID)
		if err != nil {
			return 0, false, err
		}
		if compatible && match == 0 {
			match = childID
			continue
		}
		if r.particleIsRequired(child) {
			return 0, false, nil
		}
	}
	return match, match != 0, nil
}

func (r *docResolver) choiceRestrictsElement(base Particle, restriction Particle) (bool, error) {
	baseElem, ok := r.emittedElement(base.Element)
	if !ok {
		return false, nil
	}
	for _, childID := range restriction.Children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return false, err
		}
		if particleIsExcluded(child) {
			continue
		}
		if child.Kind != ParticleElement {
			return false, nil
		}
		elem, ok := r.emittedElement(child.Element)
		if !ok {
			return false, nil
		}
		if elem.Name == baseElem.Name {
			continue
		}
		if !r.elementSubstitutesFor(child.Element, base.Element) {
			return false, nil
		}
	}
	return true, nil
}

func (r *docResolver) elementSubstitutesFor(memberID, headID ElementID) bool {
	for memberID != 0 {
		elem, ok := r.emittedElement(memberID)
		if !ok || elem.SubstitutionHead == 0 {
			return false
		}
		if elem.SubstitutionHead == headID {
			head, ok := r.emittedElement(headID)
			return ok && head.Block&ElementBlockSubstitution == 0
		}
		head, ok := r.emittedElement(elem.SubstitutionHead)
		if !ok || head.Block&ElementBlockSubstitution != 0 {
			return false
		}
		memberID = elem.SubstitutionHead
	}
	return false
}

func (r *docResolver) validatePendingParticleRestrictions() error {
	for _, check := range r.particleChecks {
		plan, ok := r.complexPlan(check.base.ID)
		if !ok || plan.Particle == 0 || check.restriction == 0 {
			continue
		}
		if err := r.validateParticleRestriction(plan.Particle, check.restriction); err != nil {
			return err
		}
	}
	r.particleChecks = nil
	return nil
}

func (r *docResolver) complexPlan(typeID TypeID) (ComplexTypePlan, bool) {
	for _, plan := range r.out.ComplexTypes {
		if plan.TypeDecl == typeID {
			return plan, true
		}
	}
	return ComplexTypePlan{}, false
}

func (r *docResolver) validateChoiceSubsetRestriction(base, restriction Particle) error {
	if base.Group != GroupChoice || len(restriction.Children) >= len(base.Children) {
		return nil
	}
	for _, childID := range restriction.Children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return err
		}
		if particleIsExcluded(child) {
			continue
		}
		if err := validateDocumentOccurrenceRestriction(base.Min, base.Max, child.Min, child.Max); err != nil {
			return err
		}
	}
	return nil
}

func particleIsExcluded(p Particle) bool {
	return !p.Max.Unbounded && p.Max.Value == 0
}

func multiplyOccurs(a, b Occurs) Occurs {
	if a.Unbounded || b.Unbounded {
		if (!a.Unbounded && a.Value == 0) || (!b.Unbounded && b.Value == 0) {
			return Occurs{}
		}
		return Occurs{Unbounded: true}
	}
	return Occurs{Value: a.Value * b.Value}
}

func addOccurs(a, b Occurs) Occurs {
	if a.Unbounded || b.Unbounded {
		return Occurs{Unbounded: true}
	}
	return Occurs{Value: a.Value + b.Value}
}

func maxOccurs(a, b Occurs) Occurs {
	if a.Unbounded || b.Unbounded {
		return Occurs{Unbounded: true}
	}
	return Occurs{Value: max(a.Value, b.Value)}
}

func (r *docResolver) particleIsRequired(p Particle) bool {
	return !r.particleCanBeEmpty(p)
}

func (r *docResolver) particleCanBeEmpty(p Particle) bool {
	if !p.Max.Unbounded && p.Max.Value == 0 {
		return true
	}
	if p.Min.Value == 0 {
		return true
	}
	if p.Kind != ParticleGroup {
		return false
	}
	switch p.Group {
	case GroupChoice:
		for _, childID := range p.Children {
			child, ok, err := r.particle(childID)
			if err != nil || !ok {
				continue
			}
			if r.particleCanBeEmpty(child) {
				return true
			}
		}
		return len(p.Children) == 0
	default:
		for _, childID := range p.Children {
			child, ok, err := r.particle(childID)
			if err != nil || !ok {
				return false
			}
			if !r.particleCanBeEmpty(child) {
				return false
			}
		}
		return true
	}
}

func (r *docResolver) validateWildcardParticleRestriction(base, restriction Particle) error {
	if particleIsExcluded(restriction) {
		return validateDocumentOccurrenceRestriction(base.Min, base.Max, restriction.Min, restriction.Max)
	}
	switch restriction.Kind {
	case ParticleWildcard:
		return r.validateWildcardToWildcardRestriction(base.Wildcard, restriction.Wildcard)
	case ParticleElement:
		elem, ok := r.emittedElement(restriction.Element)
		if !ok {
			return nil
		}
		if !r.wildcardAllowsNamespace(base.Wildcard, elem.Name.Namespace) {
			return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", elem.Name.Namespace)
		}
	}
	return nil
}

func (r *docResolver) validateWildcardToGroupParticleRestriction(base, restriction Particle) error {
	if err := r.validateWildcardNamespaceParticleRestriction(base.Wildcard, restriction); err != nil {
		return err
	}
	minOcc, maxOcc, err := r.effectiveParticleOccurrence(restriction)
	if err != nil {
		return err
	}
	return validateDocumentOccurrenceRestriction(base.Min, base.Max, minOcc, maxOcc)
}

func (r *docResolver) validateWildcardNamespaceParticleRestriction(baseID WildcardID, restriction Particle) error {
	if particleIsExcluded(restriction) {
		return nil
	}
	switch restriction.Kind {
	case ParticleElement:
		elem, ok := r.emittedElement(restriction.Element)
		if !ok {
			return nil
		}
		if !r.wildcardAllowsNamespace(baseID, elem.Name.Namespace) {
			return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", elem.Name.Namespace)
		}
	case ParticleWildcard:
		return r.validateWildcardToWildcardRestriction(baseID, restriction.Wildcard)
	case ParticleGroup:
		for _, childID := range restriction.Children {
			child, ok, err := r.particle(childID)
			if err != nil || !ok {
				return err
			}
			if err := r.validateWildcardNamespaceParticleRestriction(baseID, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *docResolver) effectiveParticleOccurrence(p Particle) (Occurs, Occurs, error) {
	if particleIsExcluded(p) {
		return Occurs{}, Occurs{}, nil
	}
	if p.Kind != ParticleGroup {
		return p.Min, p.Max, nil
	}
	if len(p.Children) == 0 {
		return Occurs{}, Occurs{}, nil
	}
	switch p.Group {
	case GroupChoice:
		var minOcc Occurs
		var minSet bool
		var maxOcc Occurs
		for _, childID := range p.Children {
			child, ok, err := r.particle(childID)
			if err != nil || !ok {
				return Occurs{}, Occurs{}, err
			}
			childMin, childMax, err := r.effectiveParticleOccurrence(child)
			if err != nil {
				return Occurs{}, Occurs{}, err
			}
			if !childMax.Unbounded && childMax.Value == 0 {
				continue
			}
			if !minSet || childMin.Value < minOcc.Value {
				minOcc = childMin
				minSet = true
			}
			maxOcc = maxOccurs(maxOcc, childMax)
		}
		return multiplyOccurs(p.Min, minOcc), multiplyOccurs(p.Max, maxOcc), nil
	default:
		var minOcc Occurs
		var maxOcc Occurs
		for _, childID := range p.Children {
			child, ok, err := r.particle(childID)
			if err != nil || !ok {
				return Occurs{}, Occurs{}, err
			}
			childMin, childMax, err := r.effectiveParticleOccurrence(child)
			if err != nil {
				return Occurs{}, Occurs{}, err
			}
			minOcc = addOccurs(minOcc, childMin)
			maxOcc = addOccurs(maxOcc, childMax)
		}
		return multiplyOccurs(p.Min, minOcc), multiplyOccurs(p.Max, maxOcc), nil
	}
}

func (r *docResolver) validateWildcardToWildcardRestriction(baseID, restrictionID WildcardID) error {
	base, ok, err := r.wildcard(baseID)
	if err != nil || !ok {
		return err
	}
	restriction, ok, err := r.wildcard(restrictionID)
	if err != nil || !ok {
		return err
	}
	baseProcess := astProcessContents(base.ProcessContents)
	restrictionProcess := astProcessContents(restriction.ProcessContents)
	if !ast.ProcessContentsStrongerOrEqual(restrictionProcess, baseProcess) {
		return fmt.Errorf(
			"ComplexContent restriction: wildcard restriction: processContents in restriction must be identical or stronger than base (base is %s, restriction is %s)",
			processContentsName(baseProcess),
			processContentsName(restrictionProcess),
		)
	}
	if !ast.NamespaceConstraintSubset(
		astNamespaceKind(restriction.NamespaceKind),
		astNamespaceList(restriction.Namespaces),
		ast.NamespaceURI(restriction.TargetNamespace),
		astNamespaceKind(base.NamespaceKind),
		astNamespaceList(base.Namespaces),
		ast.NamespaceURI(base.TargetNamespace),
	) {
		return fmt.Errorf("ComplexContent restriction: wildcard restriction: wildcard is not a subset of base wildcard")
	}
	return nil
}

func (r *docResolver) wildcard(id WildcardID) (Wildcard, bool, error) {
	if id == 0 {
		return Wildcard{}, false, nil
	}
	if int(id) > len(r.out.Wildcards) {
		return Wildcard{}, false, fmt.Errorf("schema ir: wildcard %d not found", id)
	}
	return r.out.Wildcards[id-1], true, nil
}

func (r *docResolver) wildcardAllowsNamespace(id WildcardID, namespace string) bool {
	wildcard, ok, err := r.wildcard(id)
	if err != nil || !ok {
		return false
	}
	return ast.AllowsNamespace(
		astNamespaceKind(wildcard.NamespaceKind),
		astNamespaceList(wildcard.Namespaces),
		ast.NamespaceURI(wildcard.TargetNamespace),
		ast.NamespaceURI(namespace),
	)
}

func astNamespaceList(namespaces []string) []ast.NamespaceURI {
	out := make([]ast.NamespaceURI, 0, len(namespaces))
	for _, namespace := range namespaces {
		out = append(out, ast.NamespaceURI(namespace))
	}
	return out
}

func (r *docResolver) particle(id ParticleID) (Particle, bool, error) {
	if id == 0 {
		return Particle{}, false, nil
	}
	if int(id) > len(r.out.Particles) {
		return Particle{}, false, fmt.Errorf("schema ir: particle %d not found", id)
	}
	return r.out.Particles[id-1], true, nil
}

func validateDocumentOccurrenceRestriction(baseMin, baseMax, restrictionMin, restrictionMax Occurs) error {
	if restrictionMin.Value < baseMin.Value {
		return fmt.Errorf("ComplexContent restriction: minOccurs (%s) must be >= base minOccurs (%s)",
			formatOccurs(restrictionMin), formatOccurs(baseMin))
	}
	if !baseMax.Unbounded {
		if restrictionMax.Unbounded {
			return fmt.Errorf("ComplexContent restriction: maxOccurs cannot be unbounded when base maxOccurs is bounded (%s)",
				formatOccurs(baseMax))
		}
		if restrictionMax.Value > baseMax.Value {
			return fmt.Errorf("ComplexContent restriction: maxOccurs (%s) must be <= base maxOccurs (%s)",
				formatOccurs(restrictionMax), formatOccurs(baseMax))
		}
		if restrictionMin.Value > baseMax.Value {
			return fmt.Errorf("ComplexContent restriction: minOccurs (%s) must be <= base maxOccurs (%s)",
				formatOccurs(restrictionMin), formatOccurs(baseMax))
		}
	}
	return nil
}

func formatOccurs(o Occurs) string {
	if o.Unbounded {
		return "unbounded"
	}
	return fmt.Sprintf("%d", o.Value)
}
