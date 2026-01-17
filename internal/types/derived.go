package types

// DerivedType is implemented by types that can be derived from a base type.
type DerivedType interface {
	Type

	// ResolvedBaseType returns the resolved base type, or nil if at root.
	ResolvedBaseType() Type
}

// IsDerivedFrom returns true if derived is derived (directly or indirectly) from base.
func IsDerivedFrom(derived, base Type) bool {
	if base == nil || derived == nil {
		return false
	}

	dt, ok := as[DerivedType](derived)
	if !ok {
		return false
	}

	current := dt.ResolvedBaseType()
	for current != nil {
		if current == base {
			return true
		}

		if nextDT, ok := as[DerivedType](current); ok {
			current = nextDT.ResolvedBaseType()
		} else {
			break
		}
	}

	return false
}

// IsValidlyDerivedFrom returns true when particle restriction rules allow the derivation.
// It extends IsDerivedFrom to handle unions and QName equality checks.
func IsValidlyDerivedFrom(derived, base Type) bool {
	for _, rule := range derivationRules() {
		if rule.when(derived, base) {
			return rule.then(derived, base)
		}
	}

	return false
}

// GetDerivationChain returns the chain of base types from t to its ultimate base.
// Returns an empty slice for primitive types or types without base types.
func GetDerivationChain(t Type) []Type {
	if t == nil {
		return nil
	}

	dt, ok := as[DerivedType](t)
	if !ok {
		return []Type{}
	}

	chain := make([]Type, 0)
	current := dt.ResolvedBaseType()
	for current != nil {
		chain = append(chain, current)

		if nextDT, ok := as[DerivedType](current); ok {
			current = nextDT.ResolvedBaseType()
		} else {
			break
		}
	}

	return chain
}

type derivationRule struct {
	when func(derived, base Type) bool
	then func(derived, base Type) bool
	name string
}

func derivationRules() []derivationRule {
	return []derivationRule{
		{
			name: "missing derivation inputs",
			when: isMissingDerivationInput,
			then: alwaysFalse,
		},
		{
			name: "same QName",
			when: hasSameQName,
			then: alwaysTrue,
		},
		{
			name: "standard derivation chain",
			when: isStandardDerivation,
			then: alwaysTrue,
		},
		{
			name: "union member exact match",
			when: isUnionMemberType,
			then: alwaysTrue,
		},
		{
			name: "union member derivation",
			when: isDerivedFromUnionMember,
			then: alwaysTrue,
		},
		{
			name: "no matching derivation rule",
			when: alwaysTrue,
			then: alwaysFalse,
		},
	}
}

func isMissingDerivationInput(derived, base Type) bool {
	return derived == nil || base == nil
}

func hasSameQName(derived, base Type) bool {
	return derived.Name() == base.Name()
}

func isStandardDerivation(derived, base Type) bool {
	return IsDerivedFrom(derived, base)
}

func isUnionMemberType(derived, base Type) bool {
	return matchesUnionMember(derived, base, sameQName)
}

func isDerivedFromUnionMember(derived, base Type) bool {
	return matchesUnionMember(derived, base, IsDerivedFrom)
}

func matchesUnionMember(derived, base Type, matches func(derived, base Type) bool) bool {
	for _, memberType := range unionMemberTypes(base) {
		if matches(derived, memberType) {
			return true
		}
	}

	return false
}

func unionMemberTypes(base Type) []Type {
	baseST, ok := as[*SimpleType](base)
	if !ok || baseST.Variety() != UnionVariety || len(baseST.MemberTypes) == 0 {
		return nil
	}

	return baseST.MemberTypes
}

func sameQName(derived, base Type) bool {
	return derived.Name() == base.Name()
}

func alwaysTrue(Type, Type) bool {
	return true
}

func alwaysFalse(Type, Type) bool {
	return false
}
