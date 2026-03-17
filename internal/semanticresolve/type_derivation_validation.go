package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateTypeReferences validates all type references within a type definition.
func validateTypeReferences(schema *parser.Schema, qname model.QName, typ model.Type) error {
	switch t := typ.(type) {
	case *model.SimpleType:
		if t.Restriction != nil {
			if !t.Restriction.Base.IsZero() {
				if err := validateTypeQNameReference(schema, t.Restriction.Base, qname.Namespace); err != nil {
					return fmt.Errorf("restriction base type: %w", err)
				}
				if err := validateSimpleTypeFinal(
					schema,
					t.Restriction.Base,
					model.DerivationRestriction,
					"cannot derive by restriction from type '%s' which is final for restriction",
				); err != nil {
					return err
				}
			}
			if t.ResolvedBase != nil {
				if baseST, ok := model.AsSimpleType(t.ResolvedBase); ok && baseST.Final.Has(model.DerivationRestriction) {
					return fmt.Errorf("cannot derive by restriction from type '%s' which is final for restriction", baseST.QName)
				}
			}
		}
		if t.List != nil {
			if err := validateTypeQNameReference(schema, t.List.ItemType, qname.Namespace); err != nil {
				return fmt.Errorf("list itemType: %w", err)
			}
			if err := validateSimpleTypeFinal(
				schema,
				t.List.ItemType,
				model.DerivationList,
				"cannot use type '%s' as list item type because it is final for list",
			); err != nil {
				return err
			}
		}
		if t.Union != nil {
			for i, memberType := range t.Union.MemberTypes {
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
		}
	case *model.ComplexType:
		if cc, ok := t.Content().(*model.ComplexContent); ok {
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
		}
		if sc, ok := t.Content().(*model.SimpleContent); ok {
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
		}
		if err := validateAttributeValueConstraintsForType(schema, t); err != nil {
			return err
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
