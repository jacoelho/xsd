package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
)

// typesAreEqual checks if a QName refers to the same type.
func typesAreEqual(qname model.QName, typ model.Type) bool {
	return typ.Name() == qname
}

// isTypeInDerivationChain checks if the given QName is anywhere in the derivation chain of the target type.
func isTypeInDerivationChain(sch *parser.Schema, qname model.QName, targetType model.Type) bool {
	targetQName := targetType.Name()

	current := qname
	visited := make(map[model.QName]bool)

	for !current.IsZero() && !visited[current] {
		visited[current] = true

		if current == targetQName {
			return true
		}

		typeDef, ok := sch.TypeDefs[current]
		if !ok {
			return false
		}

		ct, ok := typeDef.(*model.ComplexType)
		if !ok {
			return false
		}

		current = ct.Content().BaseTypeQName()
	}

	return false
}

func typesMatch(a, b model.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}
	nameA := a.Name()
	nameB := b.Name()
	return !nameA.IsZero() && nameA == nameB
}

func derivationStep(sch *parser.Schema, typ model.Type) (model.Type, model.DerivationMethod, error) {
	switch typed := typ.(type) {
	case *model.BuiltinType:
		name := typed.Name().Local
		if name == string(model.TypeNameAnyType) {
			return nil, 0, nil
		}
		if name == string(model.TypeNameAnySimpleType) {
			return builtins.Get(builtins.TypeNameAnyType), model.DerivationRestriction, nil
		}
		if st, ok := model.AsSimpleType(typed); ok && st.List != nil {
			return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationList, nil
		}
		return typed.BaseType(), model.DerivationRestriction, nil
	case *model.ComplexType:
		if typed.DerivationMethod == 0 {
			return typed.ResolvedBase, 0, nil
		}
		base := typed.ResolvedBase
		if base == nil {
			baseQName := typed.Content().BaseTypeQName()
			if !baseQName.IsZero() {
				resolved, err := typeops.ResolveTypeQName(sch, baseQName, typeops.TypeReferenceMustExist)
				if err != nil {
					return nil, typed.DerivationMethod, err
				}
				base = resolved
			}
		}
		return base, typed.DerivationMethod, nil
	case *model.SimpleType:
		if typed.List != nil {
			return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationList, nil
		}
		if typed.Union != nil {
			return builtins.Get(builtins.TypeNameAnySimpleType), model.DerivationUnion, nil
		}
		if typed.Restriction != nil {
			base := typed.ResolvedBase
			if base == nil && typed.Restriction.SimpleType != nil {
				base = typed.Restriction.SimpleType
			}
			if base == nil && !typed.Restriction.Base.IsZero() {
				resolved, err := typeops.ResolveTypeQName(sch, typed.Restriction.Base, typeops.TypeReferenceMustExist)
				if err != nil {
					return nil, model.DerivationRestriction, err
				}
				base = resolved
			}
			return base, model.DerivationRestriction, nil
		}
	}
	return nil, 0, nil
}

func derivationMethodLabel(method model.DerivationMethod) string {
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
