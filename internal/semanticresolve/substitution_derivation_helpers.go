package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/substpolicy"
	"github.com/jacoelho/xsd/internal/typeresolve"
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
	return substpolicy.NextDerivationStep(typ, func(name types.QName) (types.Type, error) {
		return typeresolve.ResolveTypeQName(sch, name, typeresolve.TypeReferenceMustExist)
	})
}
