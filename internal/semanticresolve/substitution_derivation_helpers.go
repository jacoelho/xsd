package semanticresolve

import (
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/substpolicy"
	"github.com/jacoelho/xsd/internal/typeresolve"
	model "github.com/jacoelho/xsd/internal/types"
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
	return substpolicy.NextDerivationStep(typ, func(name model.QName) (model.Type, error) {
		return typeresolve.ResolveTypeQName(sch, name, typeresolve.TypeReferenceMustExist)
	})
}
