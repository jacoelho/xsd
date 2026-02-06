package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func isSubstitutableElement(schema *parser.Schema, head, member types.QName) bool {
	if schema == nil || head == member {
		return true
	}
	headDecl := schema.ElementDecls[head]
	if headDecl == nil {
		return false
	}
	if headDecl.Block.Has(types.DerivationSubstitution) {
		return false
	}
	if !isSubstitutionGroupMember(schema, head, member) {
		return false
	}
	memberDecl := schema.ElementDecls[member]
	if memberDecl == nil {
		return false
	}
	headType := resolveTypeForFinalValidation(schema, headDecl.Type)
	memberType := resolveTypeForFinalValidation(schema, memberDecl.Type)
	if headType == nil || memberType == nil {
		return true
	}
	combinedBlock := headDecl.Block
	if headCT, ok := headType.(*types.ComplexType); ok {
		combinedBlock = combinedBlock.Add(types.DerivationMethod(headCT.Block))
	}
	if isDerivationBlocked(memberType, headType, combinedBlock) {
		return false
	}
	return true
}

func isSubstitutionGroupMember(schema *parser.Schema, head, member types.QName) bool {
	if schema == nil {
		return false
	}
	visited := make(map[types.QName]bool)
	var walk func(types.QName) bool
	walk = func(current types.QName) bool {
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, sub := range schema.SubstitutionGroups[current] {
			if sub == member {
				return true
			}
			if walk(sub) {
				return true
			}
		}
		return false
	}
	return walk(head)
}

func isDerivationBlocked(memberType, headType types.Type, block types.DerivationSet) bool {
	if memberType == nil || headType == nil || block == 0 {
		return false
	}
	current := memberType
	for current != nil && current != headType {
		method := derivationMethodForType(current)
		if method != 0 && block.Has(method) {
			return true
		}
		derived, ok := types.AsDerivedType(current)
		if !ok {
			return false
		}
		current = derived.ResolvedBaseType()
	}
	return false
}

func derivationMethodForType(typ types.Type) types.DerivationMethod {
	switch typed := typ.(type) {
	case *types.ComplexType:
		return typed.DerivationMethod
	case *types.SimpleType:
		if typed.List != nil || typed.Variety() == types.ListVariety {
			return types.DerivationList
		}
		if typed.Union != nil || typed.Variety() == types.UnionVariety {
			return types.DerivationUnion
		}
		if typed.Restriction != nil || typed.ResolvedBase != nil {
			return types.DerivationRestriction
		}
	case *types.BuiltinType:
		return types.DerivationRestriction
	}
	return 0
}

func isRestrictionDerivedFrom(schema *parser.Schema, derived, base types.Type) bool {
	if derived == nil || base == nil {
		return false
	}
	if baseCT, ok := base.(*types.ComplexType); ok {
		return isRestrictionDerivedFromComplex(schema, derived, baseCT)
	}
	if baseST, ok := base.(*types.SimpleType); ok && baseST.Variety() == types.UnionVariety {
		if unionAllowsDerived(schema, derived, baseST) {
			return true
		}
	}
	if types.IsValidlyDerivedFrom(derived, base) {
		return true
	}
	if derivedST, ok := derived.(*types.SimpleType); ok {
		return isRestrictionDerivedFromSimple(schema, derivedST, base)
	}
	return false
}

func isRestrictionDerivedFromComplex(schema *parser.Schema, derived types.Type, base *types.ComplexType) bool {
	derivedCT, ok := derived.(*types.ComplexType)
	if !ok {
		return false
	}
	if derivedCT == base {
		return true
	}
	visited := make(map[*types.ComplexType]bool)
	current := derivedCT
	for current != nil && !visited[current] {
		visited[current] = true
		if current == base {
			return true
		}
		if current.DerivationMethod != types.DerivationRestriction {
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
		nextCT, ok := next.(*types.ComplexType)
		if !ok {
			return false
		}
		current = nextCT
	}
	return false
}

func isRestrictionDerivedFromSimple(schema *parser.Schema, derived *types.SimpleType, base types.Type) bool {
	if derived == nil || base == nil {
		return false
	}
	if sameTypeOrQName(derived, base) {
		return true
	}
	baseName := base.Name()
	if baseName.Namespace == types.XSDNamespace && baseName.Local == string(types.TypeNameAnySimpleType) {
		return true
	}
	visited := make(map[*types.SimpleType]bool)
	current := derived
	for current != nil && !visited[current] {
		visited[current] = true
		if sameTypeOrQName(current, base) {
			return true
		}
		if current.ResolvedBase != nil {
			if sameTypeOrQName(current.ResolvedBase, base) || types.IsValidlyDerivedFrom(current.ResolvedBase, base) {
				return true
			}
			nextST, ok := current.ResolvedBase.(*types.SimpleType)
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
		if sameTypeOrQName(next, base) || types.IsValidlyDerivedFrom(next, base) {
			return true
		}
		nextST, ok := next.(*types.SimpleType)
		if !ok {
			return false
		}
		current = nextST
	}
	return false
}

func unionAllowsDerived(schema *parser.Schema, derived types.Type, baseUnion *types.SimpleType) bool {
	if derived == nil || baseUnion == nil {
		return false
	}
	if types.IsValidlyDerivedFrom(derived, baseUnion) {
		return true
	}
	members := baseUnion.MemberTypes
	if len(members) == 0 && baseUnion.Union != nil {
		members = make([]types.Type, 0, len(baseUnion.Union.MemberTypes)+len(baseUnion.Union.InlineTypes))
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
		if types.IsValidlyDerivedFrom(derived, member) || sameTypeOrQName(derived, member) {
			return true
		}
	}
	return false
}

func resolveTypeByQName(schema *parser.Schema, qname types.QName) types.Type {
	if qname.IsZero() {
		return nil
	}
	if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return bt
	}
	if schema == nil {
		return nil
	}
	if def, ok := lookupTypeDef(schema, qname); ok {
		return def
	}
	return nil
}

func sameTypeOrQName(a, b types.Type) bool {
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
