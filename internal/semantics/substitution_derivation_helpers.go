package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
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
	return model.NextDerivationStep(typ, func(name model.QName) (model.Type, error) {
		return parser.ResolveTypeQName(sch, name)
	})
}

func resolveSubstitutionTypes(sch *parser.Schema, memberDecl, headDecl *model.ElementDecl) (memberType, headType model.Type, ok bool) {
	if memberDecl == nil || headDecl == nil || memberDecl.Type == nil || headDecl.Type == nil {
		return nil, nil, false
	}
	memberType = parser.ResolveTypeReferenceAllowMissing(sch, memberDecl.Type)
	headType = parser.ResolveTypeReferenceAllowMissing(sch, headDecl.Type)
	if memberType == nil || headType == nil {
		return nil, nil, false
	}
	return memberType, headType, true
}
