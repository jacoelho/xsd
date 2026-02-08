package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// typesAreEqual checks if a QName refers to the same type.
func typesAreEqual(qname types.QName, typ types.Type) bool {
	return typ.Name() == qname
}

// isTypeInDerivationChain checks if the given QName is anywhere in the derivation chain of the target type.
func isTypeInDerivationChain(sch *parser.Schema, qname types.QName, targetType types.Type) bool {
	targetQName := targetType.Name()

	current := qname
	visited := make(map[types.QName]bool)

	for !current.IsZero() && !visited[current] {
		visited[current] = true

		if current == targetQName {
			return true
		}

		typeDef, ok := sch.TypeDefs[current]
		if !ok {
			return false
		}

		ct, ok := typeDef.(*types.ComplexType)
		if !ok {
			return false
		}

		current = ct.Content().BaseTypeQName()
	}

	return false
}

func typesMatch(a, b types.Type) bool {
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

func derivationStep(sch *parser.Schema, typ types.Type) (types.Type, types.DerivationMethod, error) {
	switch typed := typ.(type) {
	case *types.BuiltinType:
		name := typed.Name().Local
		if name == string(types.TypeNameAnyType) {
			return nil, 0, nil
		}
		if name == string(types.TypeNameAnySimpleType) {
			return types.GetBuiltin(types.TypeNameAnyType), types.DerivationRestriction, nil
		}
		if st, ok := types.AsSimpleType(typed); ok && st.List != nil {
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationList, nil
		}
		return typed.BaseType(), types.DerivationRestriction, nil
	case *types.ComplexType:
		if typed.DerivationMethod == 0 {
			return typed.ResolvedBase, 0, nil
		}
		base := typed.ResolvedBase
		if base == nil {
			baseQName := typed.Content().BaseTypeQName()
			if !baseQName.IsZero() {
				resolved, err := lookupTypeInSchema(sch, baseQName)
				if err != nil {
					return nil, typed.DerivationMethod, err
				}
				base = resolved
			}
		}
		return base, typed.DerivationMethod, nil
	case *types.SimpleType:
		if typed.List != nil {
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationList, nil
		}
		if typed.Union != nil {
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationUnion, nil
		}
		if typed.Restriction != nil {
			base := typed.ResolvedBase
			if base == nil && typed.Restriction.SimpleType != nil {
				base = typed.Restriction.SimpleType
			}
			if base == nil && !typed.Restriction.Base.IsZero() {
				resolved, err := lookupTypeInSchema(sch, typed.Restriction.Base)
				if err != nil {
					return nil, types.DerivationRestriction, err
				}
				base = resolved
			}
			return base, types.DerivationRestriction, nil
		}
	}
	return nil, 0, nil
}

func derivationMethodLabel(method types.DerivationMethod) string {
	switch method {
	case types.DerivationExtension:
		return "extension"
	case types.DerivationRestriction:
		return "restriction"
	case types.DerivationList:
		return "list"
	case types.DerivationUnion:
		return "union"
	default:
		return "unknown"
	}
}
