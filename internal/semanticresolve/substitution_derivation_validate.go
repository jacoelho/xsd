package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// validateSubstitutionGroupDerivation validates that the member element's type is derived from the head element's type.
func validateSubstitutionGroupDerivation(sch *parser.Schema, memberQName model.QName, memberDecl, headDecl *model.ElementDecl) error {
	if memberDecl != nil &&
		!memberDecl.TypeExplicit &&
		memberDecl.Type != nil &&
		model.IsAnyTypeQName(memberDecl.Type.Name()) &&
		headDecl.Type != nil {
		memberDecl.Type = headDecl.Type
	}

	memberType := typeresolve.ResolveTypeReference(sch, memberDecl.Type, typeresolve.TypeReferenceAllowMissing)
	headType := typeresolve.ResolveTypeReference(sch, headDecl.Type, typeresolve.TypeReferenceAllowMissing)
	if memberType == nil || headType == nil {
		return nil
	}
	if !memberDecl.SubstitutionGroup.IsZero() &&
		!memberDecl.TypeExplicit &&
		memberDecl.Type != nil &&
		model.IsAnyTypeQName(memberDecl.Type.Name()) {
		memberType = headType
	}

	if memberDecl.TypeExplicit && memberDecl.Type != nil {
		memberTypeName := memberDecl.Type.Name()
		if memberTypeName.Namespace == model.XSDNamespace && memberTypeName.Local == "anyType" {
			headIsAnyType := false
			headTypeName := headType.Name()
			headIsAnyType = headTypeName.Namespace == model.XSDNamespace && headTypeName.Local == "anyType"
			if !headIsAnyType && headDecl.Type != nil {
				headTypeName = headDecl.Type.Name()
				headIsAnyType = headTypeName.Namespace == model.XSDNamespace && headTypeName.Local == "anyType"
			}

			if !headIsAnyType {
				return fmt.Errorf("element %s: type '%s' is not derived from substitution group head type '%s'", memberQName, memberTypeName, headTypeName)
			}
			return nil
		}
	}

	if headType.Name().Namespace == model.XSDNamespace && headType.Name().Local == "anyType" {
		return nil
	}

	if headType.Name().Namespace == model.XSDNamespace && headType.Name().Local == "anySimpleType" {
		switch memberType.(type) {
		case *model.SimpleType, *model.BuiltinType:
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
	if !derivedValid && !model.IsValidlyDerivedFrom(memberType, headType) {
		if memberCT, ok := memberType.(*model.ComplexType); ok {
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
