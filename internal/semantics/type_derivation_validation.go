package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateTypeReferences validates all type references within a type definition.
func validateTypeReferences(schema *parser.Schema, qname model.QName, typ model.Type) error {
	switch t := typ.(type) {
	case *model.SimpleType:
		return validateSimpleTypeReferences(schema, qname, t)
	case *model.ComplexType:
		if err := validateDerivationComplexTypeReferences(schema, qname, t); err != nil {
			return err
		}
		return validateAttributeValueConstraintsForType(schema, t)
	}

	return nil
}

func validateSimpleTypeReferences(schema *parser.Schema, qname model.QName, st *model.SimpleType) error {
	if err := validateSimpleTypeRestrictionReference(schema, qname, st); err != nil {
		return err
	}
	if err := validateSimpleTypeListReference(schema, qname, st); err != nil {
		return err
	}
	return validateSimpleTypeUnionReferences(schema, qname, st)
}

func validateSimpleTypeRestrictionReference(schema *parser.Schema, qname model.QName, st *model.SimpleType) error {
	if st.Restriction == nil {
		return nil
	}
	if !st.Restriction.Base.IsZero() {
		if err := validateTypeQNameReference(schema, st.Restriction.Base, qname.Namespace); err != nil {
			return fmt.Errorf("restriction base type: %w", err)
		}
		if err := validateSimpleTypeFinal(
			schema,
			st.Restriction.Base,
			model.DerivationRestriction,
			"cannot derive by restriction from type '%s' which is final for restriction",
		); err != nil {
			return err
		}
	}
	if st.ResolvedBase != nil {
		if baseST, ok := model.AsSimpleType(st.ResolvedBase); ok && baseST.Final.Has(model.DerivationRestriction) {
			return fmt.Errorf("cannot derive by restriction from type '%s' which is final for restriction", baseST.QName)
		}
	}
	return nil
}

func validateSimpleTypeListReference(schema *parser.Schema, qname model.QName, st *model.SimpleType) error {
	if st.List == nil {
		return nil
	}
	if err := validateTypeQNameReference(schema, st.List.ItemType, qname.Namespace); err != nil {
		return fmt.Errorf("list itemType: %w", err)
	}
	return validateSimpleTypeFinal(
		schema,
		st.List.ItemType,
		model.DerivationList,
		"cannot use type '%s' as list item type because it is final for list",
	)
}

func validateSimpleTypeUnionReferences(schema *parser.Schema, qname model.QName, st *model.SimpleType) error {
	if st.Union == nil {
		return nil
	}
	for i, memberType := range st.Union.MemberTypes {
		if err := validateTypeQNameReference(schema, memberType, qname.Namespace); err != nil {
			return fmt.Errorf("union memberType %d: %w", i+1, err)
		}
		if err := validateSimpleTypeFinal(
			schema,
			memberType,
			model.DerivationUnion,
			"cannot use type '%s' as union member type because it is final for union",
		); err != nil {
			return fmt.Errorf("union memberType %d: %w", i+1, err)
		}
	}
	return nil
}

func validateDerivationComplexTypeReferences(schema *parser.Schema, qname model.QName, ct *model.ComplexType) error {
	if cc, ok := ct.Content().(*model.ComplexContent); ok {
		if err := validateComplexContentBaseReferences(schema, qname, cc); err != nil {
			return err
		}
	}
	if sc, ok := ct.Content().(*model.SimpleContent); ok {
		if err := validateSimpleContentBaseReferences(schema, qname, sc); err != nil {
			return err
		}
	}
	return nil
}

func validateComplexContentBaseReferences(schema *parser.Schema, qname model.QName, cc *model.ComplexContent) error {
	if cc.Extension != nil {
		if err := validateTypeQNameReference(schema, cc.Extension.Base, qname.Namespace); err != nil {
			return fmt.Errorf("extension base type: %w", err)
		}
	}
	if cc.Restriction != nil {
		if err := validateTypeQNameReference(schema, cc.Restriction.Base, qname.Namespace); err != nil {
			return fmt.Errorf("restriction base type: %w", err)
		}
	}
	return nil
}

func validateSimpleContentBaseReferences(schema *parser.Schema, qname model.QName, sc *model.SimpleContent) error {
	if sc.Extension != nil {
		if err := validateTypeQNameReference(schema, sc.Extension.Base, qname.Namespace); err != nil {
			return fmt.Errorf("extension base type: %w", err)
		}
	}
	if sc.Restriction != nil {
		if err := validateTypeQNameReference(schema, sc.Restriction.Base, qname.Namespace); err != nil {
			return fmt.Errorf("restriction base type: %w", err)
		}
	}
	return nil
}

func validateSimpleTypeFinal(schema *parser.Schema, qname model.QName, method model.DerivationMethod, errFmt string) error {
	if qname.IsZero() {
		return nil
	}

	if qname.Namespace == model.XSDNamespace {
		return nil
	}

	typ, exists := schema.TypeDefs[qname]
	if !exists {
		return nil
	}

	if st, ok := model.AsSimpleType(typ); ok {
		if st.Final.Has(method) {
			return fmt.Errorf(errFmt, qname)
		}
	}

	return nil
}
