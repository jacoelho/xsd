package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateNoCyclicSubstitutionGroups checks for cycles in substitution group chains.
func validateNoCyclicSubstitutionGroups(sch *parser.Schema) error {
	for _, startQName := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[startQName]
		if decl.SubstitutionGroup.IsZero() {
			continue
		}

		err := analysis.DetectGraphCycle(analysis.GraphCycleConfig[model.QName]{
			Starts:  []model.QName{startQName},
			Missing: analysis.GraphCycleMissingPolicyIgnore,
			Exists: func(name model.QName) bool {
				return sch.ElementDecls[name] != nil
			},
			Next: func(name model.QName) ([]model.QName, error) {
				decl, exists := sch.ElementDecls[name]
				if !exists || decl.SubstitutionGroup.IsZero() {
					return nil, nil
				}
				return []model.QName{decl.SubstitutionGroup}, nil
			},
		})
		if err != nil {
			var cycleErr analysis.GraphCycleError[model.QName]
			if errors.As(err, &cycleErr) {
				return fmt.Errorf("cyclic substitution group detected: element %s is part of a cycle", startQName)
			}
			return err
		}
	}

	return nil
}

// validateSubstitutionGroupDerivation validates that the member element's type is derived from the head element's type.
func validateSubstitutionGroupDerivation(sch *parser.Schema, memberQName model.QName, memberDecl, headDecl *model.ElementDecl) error {
	if shouldInheritHeadType(memberDecl, headDecl) {
		memberDecl.Type = headDecl.Type
	}

	memberType, headType, ok := resolveSubstitutionTypes(sch, memberDecl, headDecl)
	if !ok {
		return nil
	}
	if shouldReuseHeadType(memberDecl) {
		memberType = headType
	}

	if err := validateExplicitAnyType(memberQName, memberDecl, headDecl, headType); err != nil {
		return err
	}
	if headTypeAllowsSubstitution(headType, memberType) {
		return nil
	}
	if isValidSubstitutionDerivation(sch, memberType, headType) {
		return nil
	}
	return fmt.Errorf("element %s: type '%s' is not derived from substitution group head type '%s'",
		memberQName, memberType.Name(), headType.Name())
}

// validateSubstitutionGroupFinal validates that the member element's derivation
// method is not blocked by the head element's final attribute.
func validateSubstitutionGroupFinal(sch *parser.Schema, memberQName model.QName, memberDecl, headDecl *model.ElementDecl) error {
	if headDecl == nil || headDecl.Final == 0 {
		return nil
	}

	memberType, headType, ok := resolveSubstitutionTypes(sch, memberDecl, headDecl)
	if !ok || typesMatch(memberType, headType) {
		return nil
	}

	mask, ok, err := model.DerivationMask(memberType, headType, func(current model.Type) (model.Type, model.DerivationMethod, error) {
		return derivationStep(sch, current)
	})
	if err != nil {
		return fmt.Errorf("resolve substitution group derivation for %s: %w", memberQName, err)
	}
	if !ok {
		return nil
	}

	for _, method := range []model.DerivationMethod{
		model.DerivationExtension,
		model.DerivationRestriction,
		model.DerivationList,
		model.DerivationUnion,
	} {
		if mask&method != 0 && headDecl.Final.Has(method) {
			return fmt.Errorf("element %s cannot substitute for %s: head element is final for %s",
				memberQName, headDecl.Name, substitutionFinalMethodLabel(method))
		}
	}

	return nil
}

func substitutionFinalMethodLabel(method model.DerivationMethod) string {
	switch method {
	case model.DerivationExtension:
		return "extension"
	case model.DerivationRestriction:
		return "restriction"
	case model.DerivationList:
		return "list"
	case model.DerivationUnion:
		return "union"
	}
	return "unknown"
}

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

func shouldInheritHeadType(memberDecl, headDecl *model.ElementDecl) bool {
	if memberDecl == nil || headDecl == nil {
		return false
	}
	if memberDecl.TypeExplicit || memberDecl.Type == nil || headDecl.Type == nil {
		return false
	}
	return model.IsAnyTypeQName(memberDecl.Type.Name())
}

func shouldReuseHeadType(memberDecl *model.ElementDecl) bool {
	if memberDecl == nil || memberDecl.SubstitutionGroup.IsZero() {
		return false
	}
	if memberDecl.TypeExplicit || memberDecl.Type == nil {
		return false
	}
	return model.IsAnyTypeQName(memberDecl.Type.Name())
}

func validateExplicitAnyType(memberQName model.QName, memberDecl, headDecl *model.ElementDecl, headType model.Type) error {
	if !memberDecl.TypeExplicit || memberDecl.Type == nil {
		return nil
	}
	memberTypeName := memberDecl.Type.Name()
	if !isAnyType(memberTypeName) {
		return nil
	}
	headTypeName := headType.Name()
	if !isAnyType(headTypeName) && headDecl.Type != nil {
		headTypeName = headDecl.Type.Name()
	}
	if isAnyType(headTypeName) {
		return nil
	}
	return fmt.Errorf("element %s: type '%s' is not derived from substitution group head type '%s'", memberQName, memberTypeName, headTypeName)
}

func isAnyType(name model.QName) bool {
	return name.Namespace == model.XSDNamespace && name.Local == "anyType"
}

func isAnySimpleTypeName(name model.QName) bool {
	return name.Namespace == model.XSDNamespace && name.Local == "anySimpleType"
}

func isSimpleOrBuiltinType(typ model.Type) bool {
	switch typ.(type) {
	case *model.SimpleType, *model.BuiltinType:
		return true
	}
	return false
}

func headTypeAllowsSubstitution(headType, memberType model.Type) bool {
	if headType == nil || memberType == nil {
		return false
	}
	headTypeName := headType.Name()
	if isAnyType(headTypeName) {
		return true
	}
	return isAnySimpleTypeName(headTypeName) && isSimpleOrBuiltinType(memberType)
}

func isValidSubstitutionDerivation(sch *parser.Schema, memberType, headType model.Type) bool {
	if typesMatch(memberType, headType) || model.IsValidlyDerivedFrom(memberType, headType) {
		return true
	}
	memberCT, ok := memberType.(*model.ComplexType)
	if !ok {
		return false
	}
	baseQName := memberCT.Content().BaseTypeQName()
	return typesAreEqual(baseQName, headType) || isTypeInDerivationChain(sch, baseQName, headType)
}
