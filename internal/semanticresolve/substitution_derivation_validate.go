package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSubstitutionGroupDerivation validates that the member element's type is derived from the head element's type.
func validateSubstitutionGroupDerivation(sch *parser.Schema, memberQName types.QName, memberDecl, headDecl *types.ElementDecl) error {
	if isDefaultAnyType(memberDecl) && headDecl.Type != nil {
		memberDecl.Type = headDecl.Type
	}

	memberType := typeops.ResolveTypeReference(sch, memberDecl.Type, typeops.TypeReferenceAllowMissing)
	headType := typeops.ResolveTypeReference(sch, headDecl.Type, typeops.TypeReferenceAllowMissing)
	if memberType == nil || headType == nil {
		return nil
	}
	if !memberDecl.SubstitutionGroup.IsZero() && !memberDecl.TypeExplicit && isDefaultAnyType(memberDecl) {
		memberType = headType
	}

	if memberDecl.TypeExplicit && memberDecl.Type != nil {
		memberTypeName := memberDecl.Type.Name()
		if memberTypeName.Namespace == types.XSDNamespace && memberTypeName.Local == "anyType" {
			headIsAnyType := false
			headTypeName := headType.Name()
			headIsAnyType = headTypeName.Namespace == types.XSDNamespace && headTypeName.Local == "anyType"
			if !headIsAnyType && headDecl.Type != nil {
				headTypeName = headDecl.Type.Name()
				headIsAnyType = headTypeName.Namespace == types.XSDNamespace && headTypeName.Local == "anyType"
			}

			if !headIsAnyType {
				return fmt.Errorf("element %s: type '%s' is not derived from substitution group head type '%s'", memberQName, memberTypeName, headTypeName)
			}
			return nil
		}
	}

	if headType.Name().Namespace == types.XSDNamespace && headType.Name().Local == "anyType" {
		return nil
	}

	if headType.Name().Namespace == types.XSDNamespace && headType.Name().Local == "anySimpleType" {
		switch memberType.(type) {
		case *types.SimpleType, *types.BuiltinType:
			return nil
		}
	}

	derivedValid := memberType == headType
	if !derivedValid {
		memberName := memberType.Name()
		headName := headType.Name()
		if !memberName.IsZero() && !headName.IsZero() && memberName == headName {
			derivedValid = true
		}
	}
	if !derivedValid && !types.IsValidlyDerivedFrom(memberType, headType) {
		if memberCT, ok := memberType.(*types.ComplexType); ok {
			baseQName := memberCT.Content().BaseTypeQName()
			if typesAreEqual(baseQName, headType) || isTypeInDerivationChain(sch, baseQName, headType) {
				derivedValid = true
			}
		}
		if !derivedValid {
			return fmt.Errorf("element %s: type '%s' is not derived from substitution group head type '%s'",
				memberQName, memberType.Name(), headType.Name())
		}
	}

	return nil
}

func isDefaultAnyType(decl *types.ElementDecl) bool {
	if decl == nil || decl.TypeExplicit {
		return false
	}
	return types.IsAnyType(decl.Type)
}
