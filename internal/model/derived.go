package model

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
// Returns an empty slice for primitive types or types without base model.
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
			when: IsDerivedFrom,
			then: alwaysTrue,
		},
		{
			name: "union member exact match",
			when: func(derived, base Type) bool {
				return matchesUnionMember(derived, base, hasSameQName)
			},
			then: alwaysTrue,
		},
		{
			name: "union member derivation",
			when: func(derived, base Type) bool {
				return matchesUnionMember(derived, base, IsDerivedFrom)
			},
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
	if derived == base {
		return true
	}
	nameA := derived.Name()
	nameB := base.Name()
	if nameA.IsZero() || nameB.IsZero() {
		return false
	}
	return nameA == nameB
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
	return UnionMemberTypesWithResolver(base, nil)
}

// DerivationStepFunc returns the next base type and derivation method in a derivation chain.
type DerivationStepFunc func(Type) (Type, DerivationMethod, error)

// TypeQNameResolver resolves a type QName into a type declaration.
type TypeQNameResolver func(QName) (Type, error)

// NextDerivationStep returns the next base type and derivation method for one step.
func NextDerivationStep(current Type, resolve TypeQNameResolver) (Type, DerivationMethod, error) {
	switch typed := current.(type) {
	case *ComplexType:
		method := typed.DerivationMethod
		if method == 0 {
			method = DerivationRestriction
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, method, nil
		}
		baseQName := QName{}
		if content := typed.Content(); content != nil {
			baseQName = content.BaseTypeQName()
		}
		if !baseQName.IsZero() && resolve != nil {
			base, err := resolve(baseQName)
			if err != nil {
				return nil, method, err
			}
			return base, method, nil
		}
		return typed.BaseType(), method, nil
	case *SimpleType:
		if typed.List != nil {
			return GetBuiltin(TypeNameAnySimpleType), DerivationList, nil
		}
		if typed.Union != nil {
			return GetBuiltin(TypeNameAnySimpleType), DerivationUnion, nil
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, DerivationRestriction, nil
		}
		if typed.Restriction != nil {
			if typed.Restriction.SimpleType != nil {
				return typed.Restriction.SimpleType, DerivationRestriction, nil
			}
			if !typed.Restriction.Base.IsZero() && resolve != nil {
				base, err := resolve(typed.Restriction.Base)
				if err != nil {
					return nil, DerivationRestriction, err
				}
				return base, DerivationRestriction, nil
			}
		}
		return nil, 0, nil
	case *BuiltinType:
		name := TypeName(typed.Name().Local)
		switch name {
		case TypeNameAnyType:
			return nil, 0, nil
		case TypeNameAnySimpleType:
			return GetBuiltin(TypeNameAnyType), DerivationRestriction, nil
		default:
			if _, ok := BuiltinListItemTypeName(typed.Name().Local); ok {
				return GetBuiltin(TypeNameAnySimpleType), DerivationList, nil
			}
			base := typed.BaseType()
			if base == nil {
				return nil, 0, nil
			}
			return base, DerivationRestriction, nil
		}
	default:
		return nil, 0, nil
	}
}

// DerivationMask computes the derivation-method mask from derived to base.
func DerivationMask(derived, base Type, step DerivationStepFunc) (DerivationMethod, bool, error) {
	if derived == nil || base == nil {
		return 0, false, nil
	}
	if derived == base {
		return 0, true, nil
	}
	if step == nil {
		step = func(current Type) (Type, DerivationMethod, error) {
			return NextDerivationStep(current, nil)
		}
	}
	mask := DerivationMethod(0)
	seen := make(map[Type]bool)
	current := derived
	for current != nil && current != base {
		if seen[current] {
			break
		}
		seen[current] = true
		next, method, err := step(current)
		if err != nil {
			return 0, false, err
		}
		if next == nil {
			break
		}
		mask |= method
		current = next
	}
	if current == base {
		return mask, true, nil
	}
	return 0, false, nil
}

// BlockedDerivations computes effective blocked derivations from head element and head type.
func BlockedDerivations(head *ElementDecl) DerivationMethod {
	if head == nil {
		return 0
	}
	mask := derivationSetMask(head.Block) | derivationSetMask(head.Final)
	switch typ := head.Type.(type) {
	case *ComplexType:
		mask |= derivationSetMask(typ.Block)
		mask |= derivationSetMask(typ.Final)
	case *SimpleType:
		mask |= derivationSetMask(typ.Final)
	}
	return mask
}

func derivationSetMask(set DerivationSet) DerivationMethod {
	mask := DerivationMethod(0)
	for _, method := range []DerivationMethod{
		DerivationExtension,
		DerivationRestriction,
		DerivationList,
		DerivationUnion,
	} {
		if set.Has(method) {
			mask |= method
		}
	}
	return mask
}

func alwaysTrue(Type, Type) bool {
	return true
}

func alwaysFalse(Type, Type) bool {
	return false
}
