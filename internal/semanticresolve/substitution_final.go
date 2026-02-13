package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/substpolicy"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSubstitutionGroupFinal validates that the substitution group member's derivation
// method is not blocked by the head element's final attribute.
func validateSubstitutionGroupFinal(sch *parser.Schema, memberQName types.QName, memberDecl, headDecl *types.ElementDecl) error {
	if headDecl.Final == 0 {
		return nil
	}

	memberType := memberDecl.Type
	headType := headDecl.Type

	if memberType == nil || headType == nil {
		return nil
	}

	memberType = typeresolve.ResolveTypeReference(sch, memberType, typeresolve.TypeReferenceAllowMissing)
	headType = typeresolve.ResolveTypeReference(sch, headType, typeresolve.TypeReferenceAllowMissing)

	if memberType == nil || headType == nil {
		return nil
	}

	if typesMatch(memberType, headType) {
		return nil
	}

	mask, ok, err := substpolicy.DerivationMask(memberType, headType, func(current types.Type) (types.Type, types.DerivationMethod, error) {
		return derivationStep(sch, current)
	})
	if err != nil {
		return fmt.Errorf("resolve substitution group derivation for %s: %w", memberQName, err)
	}
	if !ok {
		return nil
	}

	for _, method := range []types.DerivationMethod{
		types.DerivationExtension,
		types.DerivationRestriction,
		types.DerivationList,
		types.DerivationUnion,
	} {
		if mask&method != 0 && headDecl.Final.Has(method) {
			return fmt.Errorf("element %s cannot substitute for %s: head element is final for %s",
				memberQName, headDecl.Name, substpolicy.MethodLabel(method))
		}
	}

	return nil
}
