package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

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
