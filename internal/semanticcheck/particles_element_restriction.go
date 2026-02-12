package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// validateElementRestriction validates that a restriction element properly restricts a base element.
// Per XSD 1.0 spec section 3.4.6 Constraints on Particle Schema Components:
// - nillable: If base is not nillable, restriction cannot be nillable
// - fixed: If base has fixed value, restriction must have same fixed value
// - block: Restriction block must be superset of base block (cannot allow more derivations)
// - type: Restriction type must be same as or derived from base type
func validateElementRestriction(schema *parser.Schema, baseElem, restrictionElem *model.ElementDecl) error {
	if !baseElem.Nillable && restrictionElem.Nillable {
		return fmt.Errorf("ComplexContent restriction: element '%s' nillable cannot be true when base element nillable is false", restrictionElem.Name)
	}

	if baseElem.HasFixed {
		if !restrictionElem.HasFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' must have fixed value matching base fixed value '%s'", restrictionElem.Name, baseElem.Fixed)
		}
		baseType := effectiveElementType(schema, baseElem)
		restrictionType := effectiveElementType(schema, restrictionElem)
		baseFixed := normalizeFixedValue(baseElem.Fixed, baseType)
		restrictionFixed := normalizeFixedValue(restrictionElem.Fixed, restrictionType)
		if baseFixed != restrictionFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' fixed value '%s' must match base fixed value '%s'", restrictionElem.Name, restrictionElem.Fixed, baseElem.Fixed)
		}
	}

	if !isBlockSuperset(restrictionElem.Block, baseElem.Block) {
		return fmt.Errorf("ComplexContent restriction: element '%s' block constraint must be superset of base block constraint", restrictionElem.Name)
	}

	baseType := effectiveElementType(schema, baseElem)
	restrictionType := effectiveElementType(schema, restrictionElem)
	if baseType == nil || restrictionType == nil {
		return nil
	}
	baseTypeName := baseType.Name()
	restrictionTypeName := restrictionType.Name()

	if baseTypeName.Namespace == model.XSDNamespace && baseTypeName.Local == "anyType" {
		return nil
	}
	if baseTypeName.Namespace == model.XSDNamespace && baseTypeName.Local == "anySimpleType" {
		switch restrictionType.(type) {
		case *model.SimpleType, *model.BuiltinType:
			return nil
		}
	}

	if baseTypeName == restrictionTypeName {
		return nil
	}

	if restrictionTypeName.Local == "" {
		if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
			if st, ok := restrictionType.(*model.SimpleType); ok {
				if st.Restriction != nil && st.Restriction.Base == baseTypeName {
					return nil
				}
				if st.ResolvedBase != nil && isRestrictionDerivedFrom(schema, st.ResolvedBase, baseType) {
					return nil
				}
			}
			return fmt.Errorf("ComplexContent restriction: element '%s' anonymous type must be derived from base type '%s'", restrictionElem.Name, baseTypeName)
		}
		return nil
	}

	if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
		return fmt.Errorf("ComplexContent restriction: element '%s' type '%s' must be same as or derived from base type '%s'", restrictionElem.Name, restrictionTypeName, baseTypeName)
	}

	return nil
}

func effectiveElementType(schema *parser.Schema, elem *model.ElementDecl) model.Type {
	if elem == nil {
		return nil
	}
	resolved := typeresolve.ResolveTypeReference(schema, elem.Type, typeresolve.TypeReferenceAllowMissing)
	if resolved != nil {
		return resolved
	}
	return elem.Type
}

func normalizeFixedValue(value string, typ model.Type) string {
	if typ == nil {
		return value
	}
	if st, ok := typ.(*model.SimpleType); ok {
		if st.List != nil || st.Variety() == model.ListVariety {
			return model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() &&
			st.Restriction.Base.Namespace == model.XSDNamespace &&
			builtins.IsBuiltinListTypeName(st.Restriction.Base.Local) {
			return model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
		}
	}
	if bt, ok := typ.(*model.BuiltinType); ok && builtins.IsBuiltinListTypeName(bt.Name().Local) {
		return model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
	}
	return model.NormalizeWhiteSpace(value, typ)
}
