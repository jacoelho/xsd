package substpolicy

import (
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
)

// DerivationStepFunc defines an exported type.
type DerivationStepFunc func(model.Type) (model.Type, model.DerivationMethod, error)

// TypeQNameResolver defines an exported type.
type TypeQNameResolver func(model.QName) (model.Type, error)

// NextDerivationStep returns the next base type and derivation method for one step.
func NextDerivationStep(current model.Type, resolve TypeQNameResolver) (model.Type, model.DerivationMethod, error) {
	switch typed := current.(type) {
	case *model.ComplexType:
		method := typed.DerivationMethod
		if method == 0 {
			method = model.DerivationRestriction
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, method, nil
		}
		baseQName := model.QName{}
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
	case *model.SimpleType:
		if typed.List != nil {
			return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationList, nil
		}
		if typed.Union != nil {
			return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationUnion, nil
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, model.DerivationRestriction, nil
		}
		if typed.Restriction != nil {
			if typed.Restriction.SimpleType != nil {
				return typed.Restriction.SimpleType, model.DerivationRestriction, nil
			}
			if !typed.Restriction.Base.IsZero() && resolve != nil {
				base, err := resolve(typed.Restriction.Base)
				if err != nil {
					return nil, model.DerivationRestriction, err
				}
				return base, model.DerivationRestriction, nil
			}
		}
		return nil, 0, nil
	case *model.BuiltinType:
		name := model.TypeName(typed.Name().Local)
		switch name {
		case builtins.TypeNameAnyType:
			return nil, 0, nil
		case builtins.TypeNameAnySimpleType:
			return builtins.Get(builtins.TypeNameAnyType), model.DerivationRestriction, nil
		default:
			if _, ok := builtins.BuiltinListItemTypeName(typed.Name().Local); ok {
				return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationList, nil
			}
			base := typed.BaseType()
			if base == nil {
				return nil, 0, nil
			}
			return base, model.DerivationRestriction, nil
		}
	default:
		return nil, 0, nil
	}
}

// DerivationMask computes the derivation-method mask from derived to base.
func DerivationMask(derived, base model.Type, step DerivationStepFunc) (model.DerivationMethod, bool, error) {
	if derived == nil || base == nil {
		return 0, false, nil
	}
	if derived == base {
		return 0, true, nil
	}
	mask := model.DerivationMethod(0)
	seen := make(map[model.Type]bool)
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
func BlockedDerivations(head *model.ElementDecl) model.DerivationMethod {
	if head == nil {
		return 0
	}
	var mask model.DerivationMethod
	if head.Block.Has(model.DerivationExtension) {
		mask |= model.DerivationExtension
	}
	if head.Block.Has(model.DerivationRestriction) {
		mask |= model.DerivationRestriction
	}
	if head.Final.Has(model.DerivationExtension) {
		mask |= model.DerivationExtension
	}
	if head.Final.Has(model.DerivationRestriction) {
		mask |= model.DerivationRestriction
	}
	if head.Final.Has(model.DerivationList) {
		mask |= model.DerivationList
	}
	if head.Final.Has(model.DerivationUnion) {
		mask |= model.DerivationUnion
	}

	switch typ := head.Type.(type) {
	case *model.ComplexType:
		if typ.Block.Has(model.DerivationExtension) {
			mask |= model.DerivationExtension
		}
		if typ.Block.Has(model.DerivationRestriction) {
			mask |= model.DerivationRestriction
		}
		if typ.Final.Has(model.DerivationExtension) {
			mask |= model.DerivationExtension
		}
		if typ.Final.Has(model.DerivationRestriction) {
			mask |= model.DerivationRestriction
		}
	case *model.SimpleType:
		if typ.Final.Has(model.DerivationRestriction) {
			mask |= model.DerivationRestriction
		}
		if typ.Final.Has(model.DerivationList) {
			mask |= model.DerivationList
		}
		if typ.Final.Has(model.DerivationUnion) {
			mask |= model.DerivationUnion
		}
	}

	return mask
}

// MethodLabel formats a derivation method for diagnostics.
func MethodLabel(method model.DerivationMethod) string {
	switch method {
	case model.DerivationExtension:
		return "extension"
	case model.DerivationRestriction:
		return "restriction"
	case model.DerivationList:
		return "list"
	case model.DerivationUnion:
		return "union"
	default:
		return "unknown"
	}
}
