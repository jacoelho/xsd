package semantics

import (
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func normalizePointlessParticle(p model.Particle) model.Particle {
	for {
		mg, ok := p.(*model.ModelGroup)
		if !ok || mg == nil {
			return p
		}
		if !mg.MinOccurs.IsOne() || !mg.MaxOccurs.IsOne() {
			return p
		}
		children := derivationChildren(mg)
		if len(children) != 1 {
			return p
		}
		p = children[0]
	}
}

func derivationChildren(mg *model.ModelGroup) []model.Particle {
	if mg == nil {
		return nil
	}
	children := make([]model.Particle, 0, len(mg.Particles))
	for _, child := range mg.Particles {
		children = append(children, gatherPointlessChildren(mg.Kind, child)...)
	}
	return children
}

func gatherPointlessChildren(parentKind model.GroupKind, particle model.Particle) []model.Particle {
	switch p := particle.(type) {
	case *model.ElementDecl, *model.AnyElement:
		return []model.Particle{p}
	case *model.ModelGroup:
		if !p.MinOccurs.IsOne() || !p.MaxOccurs.IsOne() {
			return []model.Particle{p}
		}
		if len(p.Particles) == 1 {
			return gatherPointlessChildren(parentKind, p.Particles[0])
		}
		if p.Kind == parentKind {
			var out []model.Particle
			for _, child := range p.Particles {
				out = append(out, gatherPointlessChildren(parentKind, child)...)
			}
			return out
		}
		return []model.Particle{p}
	default:
		return []model.Particle{p}
	}
}

// isBlockSuperset checks if restrictionBlock is a superset of baseBlock.
// Restriction block must contain all derivation methods in base block.
func isBlockSuperset(restrictionBlock, baseBlock model.DerivationSet) bool {
	if baseBlock.Has(model.DerivationExtension) && !restrictionBlock.Has(model.DerivationExtension) {
		return false
	}
	if baseBlock.Has(model.DerivationRestriction) && !restrictionBlock.Has(model.DerivationRestriction) {
		return false
	}
	if baseBlock.Has(model.DerivationSubstitution) && !restrictionBlock.Has(model.DerivationSubstitution) {
		return false
	}
	return true
}

// calculateEffectiveOccurrence calculates effective minOccurs and maxOccurs for a model group.
func calculateEffectiveOccurrence(mg *model.ModelGroup) (minOcc, maxOcc model.Occurs) {
	groupMinOcc := mg.MinOcc()
	groupMaxOcc := mg.MaxOcc()

	if len(mg.Particles) == 0 {
		return model.OccursFromInt(0), model.OccursFromInt(0)
	}

	switch mg.Kind {
	case model.Sequence, model.AllGroup:
		sumMinOcc := model.OccursFromInt(0)
		sumMaxOcc := model.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = model.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = model.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = model.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = model.MulOccurs(groupMaxOcc, sumMaxOcc)
	case model.Choice:
		childMinOcc := model.OccursFromInt(0)
		childMaxOcc := model.OccursFromInt(0)
		childMinOccSet := false
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			if childMax.IsZero() {
				continue
			}
			if !childMinOccSet || childMin.Cmp(childMinOcc) < 0 {
				childMinOcc = childMin
				childMinOccSet = true
			}
			childMaxOcc = model.MaxOccurs(childMaxOcc, childMax)
		}
		if !childMinOccSet {
			childMinOcc = model.OccursFromInt(0)
		}
		minOcc = model.MulOccurs(groupMinOcc, childMinOcc)
		maxOcc = model.MulOccurs(groupMaxOcc, childMaxOcc)
	default:
		minOcc = groupMinOcc
		maxOcc = groupMaxOcc
	}
	return
}

// getParticleEffectiveOccurrence gets effective occurrence for a single particle.
func getParticleEffectiveOccurrence(p model.Particle) (minOcc, maxOcc model.Occurs) {
	switch particle := p.(type) {
	case *model.ModelGroup:
		return calculateEffectiveOccurrence(particle)
	case *model.ElementDecl:
		return particle.MinOcc(), particle.MaxOcc()
	case *model.AnyElement:
		return particle.MinOccurs, particle.MaxOccurs
	default:
		return p.MinOcc(), p.MaxOcc()
	}
}

// isEffectivelyOptional checks if a model group is effectively optional.
func isEffectivelyOptional(mg *model.ModelGroup) bool {
	if len(mg.Particles) == 0 {
		return true
	}
	for _, particle := range mg.Particles {
		if particle.MinOcc().CmpInt(0) > 0 {
			return false
		}
		if nestedMG, ok := particle.(*model.ModelGroup); ok {
			if !isEffectivelyOptional(nestedMG) {
				return false
			}
		}
	}
	return true
}

// isEmptiableParticle reports whether a particle can match the empty sequence.
func isEmptiableParticle(p model.Particle) bool {
	if p == nil || p.MaxOcc().IsZero() || p.MinOcc().IsZero() {
		return true
	}

	switch pt := p.(type) {
	case *model.ModelGroup:
		switch pt.Kind {
		case model.Sequence, model.AllGroup:
			for _, child := range pt.Particles {
				if !isEmptiableParticle(child) {
					return false
				}
			}
			return true
		case model.Choice:
			return slices.ContainsFunc(pt.Particles, isEmptiableParticle)
		default:
			return false
		}
	case *model.ElementDecl, *model.AnyElement:
		return false
	default:
		return p.MinOcc().IsZero()
	}
}

func isSubstitutableElement(schema *parser.Schema, head, member model.QName) bool {
	if schema == nil || head == member {
		return true
	}
	headDecl := schema.ElementDecls[head]
	if headDecl == nil || headDecl.Block.Has(model.DerivationSubstitution) {
		return false
	}
	if !isSubstitutionGroupMember(schema, head, member) {
		return false
	}
	memberDecl := schema.ElementDecls[member]
	if memberDecl == nil {
		return false
	}
	headType := parser.ResolveTypeReferenceAllowMissing(schema, headDecl.Type)
	memberType := parser.ResolveTypeReferenceAllowMissing(schema, memberDecl.Type)
	if headType == nil || memberType == nil {
		return true
	}
	combinedBlock := headDecl.Block
	if headCT, ok := headType.(*model.ComplexType); ok {
		combinedBlock = combinedBlock.Add(model.DerivationMethod(headCT.Block))
	}
	return !isDerivationBlocked(memberType, headType, combinedBlock)
}

func isSubstitutionGroupMember(schema *parser.Schema, head, member model.QName) bool {
	if schema == nil {
		return false
	}
	visited := make(map[model.QName]bool)
	var walk func(model.QName) bool
	walk = func(current model.QName) bool {
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, sub := range schema.SubstitutionGroups[current] {
			if sub == member || walk(sub) {
				return true
			}
		}
		return false
	}
	return walk(head)
}

func isDerivationBlocked(memberType, headType model.Type, block model.DerivationSet) bool {
	if memberType == nil || headType == nil || block == 0 {
		return false
	}
	current := memberType
	for current != nil && current != headType {
		method := derivationMethodForType(current)
		if method != 0 && block.Has(method) {
			return true
		}
		derived, ok := model.AsDerivedType(current)
		if !ok {
			return false
		}
		current = derived.ResolvedBaseType()
	}
	return false
}

func derivationMethodForType(typ model.Type) model.DerivationMethod {
	switch typed := typ.(type) {
	case *model.ComplexType:
		return typed.DerivationMethod
	case *model.SimpleType:
		if typed.List != nil || typed.Variety() == model.ListVariety {
			return model.DerivationList
		}
		if typed.Union != nil || typed.Variety() == model.UnionVariety {
			return model.DerivationUnion
		}
		if typed.Restriction != nil || typed.ResolvedBase != nil {
			return model.DerivationRestriction
		}
	case *model.BuiltinType:
		return model.DerivationRestriction
	}
	return 0
}

func isRestrictionDerivedFrom(schema *parser.Schema, derived, base model.Type) bool {
	if derived == nil || base == nil {
		return false
	}
	if baseCT, ok := base.(*model.ComplexType); ok {
		return isRestrictionDerivedFromComplex(schema, derived, baseCT)
	}
	if baseST, ok := base.(*model.SimpleType); ok && baseST.Variety() == model.UnionVariety {
		if unionAllowsDerived(schema, derived, baseST) {
			return true
		}
	}
	if model.IsValidlyDerivedFrom(derived, base) {
		return true
	}
	if derivedST, ok := derived.(*model.SimpleType); ok {
		return isRestrictionDerivedFromSimple(schema, derivedST, base)
	}
	return false
}

func isRestrictionDerivedFromComplex(schema *parser.Schema, derived model.Type, base *model.ComplexType) bool {
	derivedCT, ok := derived.(*model.ComplexType)
	if !ok {
		return false
	}
	if derivedCT == base {
		return true
	}
	visited := make(map[*model.ComplexType]bool)
	current := derivedCT
	for current != nil && !visited[current] {
		visited[current] = true
		if current == base {
			return true
		}
		if current.DerivationMethod != model.DerivationRestriction {
			return false
		}
		next := current.ResolvedBase
		if next == nil {
			baseQName := current.Content().BaseTypeQName()
			if baseQName.IsZero() {
				return false
			}
			if baseQName == base.QName {
				return true
			}
			next = resolveTypeByQName(schema, baseQName)
			if next == nil {
				return false
			}
		}
		if next == base {
			return true
		}
		nextCT, ok := next.(*model.ComplexType)
		if !ok {
			return false
		}
		current = nextCT
	}
	return false
}

func isRestrictionDerivedFromSimple(schema *parser.Schema, derived *model.SimpleType, base model.Type) bool {
	if derived == nil || base == nil {
		return false
	}
	if sameTypeOrQName(derived, base) {
		return true
	}
	baseName := base.Name()
	if baseName.Namespace == model.XSDNamespace && baseName.Local == string(model.TypeNameAnySimpleType) {
		return true
	}
	visited := make(map[*model.SimpleType]bool)
	current := derived
	for current != nil && !visited[current] {
		visited[current] = true
		if sameTypeOrQName(current, base) {
			return true
		}
		if current.ResolvedBase != nil {
			if sameTypeOrQName(current.ResolvedBase, base) || model.IsValidlyDerivedFrom(current.ResolvedBase, base) {
				return true
			}
			nextST, ok := current.ResolvedBase.(*model.SimpleType)
			if !ok {
				return false
			}
			current = nextST
			continue
		}
		if current.Restriction == nil || current.Restriction.Base.IsZero() {
			return false
		}
		baseQName := current.Restriction.Base
		if baseQName == baseName {
			return true
		}
		next := resolveTypeByQName(schema, baseQName)
		if next == nil {
			return false
		}
		if sameTypeOrQName(next, base) || model.IsValidlyDerivedFrom(next, base) {
			return true
		}
		nextST, ok := next.(*model.SimpleType)
		if !ok {
			return false
		}
		current = nextST
	}
	return false
}

func unionAllowsDerived(schema *parser.Schema, derived model.Type, baseUnion *model.SimpleType) bool {
	if derived == nil || baseUnion == nil {
		return false
	}
	if model.IsValidlyDerivedFrom(derived, baseUnion) {
		return true
	}
	members := baseUnion.MemberTypes
	if len(members) == 0 && baseUnion.Union != nil {
		members = make([]model.Type, 0, len(baseUnion.Union.MemberTypes)+len(baseUnion.Union.InlineTypes))
		for _, inline := range baseUnion.Union.InlineTypes {
			members = append(members, inline)
		}
		for _, qname := range baseUnion.Union.MemberTypes {
			if member := resolveTypeByQName(schema, qname); member != nil {
				members = append(members, member)
			} else if derivedName := derived.Name(); !derivedName.IsZero() && derivedName == qname {
				return true
			}
		}
	}
	for _, member := range members {
		if member == nil {
			continue
		}
		if model.IsValidlyDerivedFrom(derived, member) || sameTypeOrQName(derived, member) {
			return true
		}
	}
	return false
}

func resolveTypeByQName(schema *parser.Schema, qname model.QName) model.Type {
	if qname.IsZero() {
		return nil
	}
	if bt := model.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return bt
	}
	if schema == nil {
		return nil
	}
	if def, ok := LookupType(schema, qname); ok {
		return def
	}
	return nil
}

func sameTypeOrQName(a, b model.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}
	nameA := a.Name()
	nameB := b.Name()
	if nameA.IsZero() || nameB.IsZero() {
		return false
	}
	return nameA == nameB
}
