package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
)

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
	if bt := builtins.GetNS(qname.Namespace, qname.Local); bt != nil {
		return bt
	}
	if schema == nil {
		return nil
	}
	if def, ok := typechain.LookupType(schema, qname); ok {
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
