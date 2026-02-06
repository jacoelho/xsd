package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

const noOriginLocation = ""

// validateTypeReferenceFromType validates that a type reference exists (from a Type interface).
func validateTypeReferenceFromType(schema *parser.Schema, typ types.Type, contextNamespace types.NamespaceURI) error {
	return validateTypeReferenceFromTypeAtLocation(schema, typ, contextNamespace, noOriginLocation)
}

func validateTypeReferenceFromTypeAtLocation(schema *parser.Schema, typ types.Type, contextNamespace types.NamespaceURI, originLocation string) error {
	visited := make(map[*types.ModelGroup]bool)
	return validateTypeReferenceFromTypeWithVisited(schema, typ, visited, contextNamespace, originLocation)
}

// validateTypeReferenceFromTypeWithVisited validates type reference with cycle detection.
func validateTypeReferenceFromTypeWithVisited(schema *parser.Schema, typ types.Type, visited map[*types.ModelGroup]bool, contextNamespace types.NamespaceURI, originLocation string) error {
	if typ == nil {
		return nil
	}
	qname := typ.Name()
	if !qname.IsZero() {
		if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
			return err
		}
	}
	// built-in types are always valid.
	if typ.IsBuiltin() {
		return nil
	}

	// check if it's a placeholder SimpleType (has QName but not builtin and no Restriction/List/Union).
	if st, ok := types.AsSimpleType(typ); ok {
		// check if it's a placeholder: not builtin, has QName, but no Restriction/List/Union.
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			// this is a placeholder - check if type exists.
			if _, exists := schema.TypeDefs[st.QName]; !exists {
				// check if it's a built-in type in xsd namespace.
				if st.QName.Namespace == types.XSDNamespace {
					if types.GetBuiltin(types.TypeName(st.QName.Local)) == nil {
						return fmt.Errorf("type '%s' not found in XSD namespace", st.QName.Local)
					}
					return nil
				}
				return fmt.Errorf("type %s not found", st.QName)
			}
		}
	}

	// check inline complex type's content model for references.
	if ct, ok := types.AsComplexType(typ); ok {
		if content := ct.Content(); content != nil {
			if ec, ok := content.(*types.ElementContent); ok && ec.Particle != nil {
				if err := validateParticleReferencesWithVisited(schema, ec.Particle, visited, originLocation); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateTypeReferences validates all type references within a type definition.
func validateTypeReferences(schema *parser.Schema, qname types.QName, typ types.Type) error {
	switch t := typ.(type) {
	case *types.SimpleType:
		if t.Restriction != nil {
			// validate base type reference (skip if Base is zero, which indicates inline simpleType base).
			if !t.Restriction.Base.IsZero() {
				if err := validateTypeQNameReference(schema, t.Restriction.Base, qname.Namespace); err != nil {
					return fmt.Errorf("restriction base type: %w", err)
				}
				// check if base type's final attribute blocks restriction derivation.
				if err := validateSimpleTypeFinalRestriction(schema, t.Restriction.Base); err != nil {
					return err
				}
			}
			// also check if we have an inline base type with final attribute.
			if t.ResolvedBase != nil {
				if baseST, ok := types.AsSimpleType(t.ResolvedBase); ok && baseST.Final.Has(types.DerivationRestriction) {
					return fmt.Errorf("cannot derive by restriction from type '%s' which is final for restriction", baseST.QName)
				}
			}
		}
		if t.List != nil {
			if err := validateTypeQNameReference(schema, t.List.ItemType, qname.Namespace); err != nil {
				return fmt.Errorf("list itemType: %w", err)
			}
			// check if item type's final attribute blocks list derivation.
			if err := validateSimpleTypeFinalList(schema, t.List.ItemType); err != nil {
				return err
			}
		}
		if t.Union != nil {
			for i, memberType := range t.Union.MemberTypes {
				if err := validateTypeQNameReference(schema, memberType, qname.Namespace); err != nil {
					return fmt.Errorf("union memberType %d: %w", i+1, err)
				}
				// check if member type's final attribute blocks union derivation.
				if err := validateSimpleTypeFinalUnion(schema, memberType); err != nil {
					return fmt.Errorf("union memberType %d: %w", i+1, err)
				}
			}
		}
	case *types.ComplexType:
		// complex types don't have direct type references, but may have base types in complexContent.
		if cc, ok := t.Content().(*types.ComplexContent); ok {
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
		if sc, ok := t.Content().(*types.SimpleContent); ok {
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

// validateTypeQNameReference validates that a type QName reference exists.
func validateTypeQNameReference(schema *parser.Schema, qname types.QName, contextNamespace types.NamespaceURI) error {
	// empty QName is not valid.
	if qname.IsZero() {
		return nil // this case is handled elsewhere (e.g., inline types).
	}

	if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
		return err
	}

	// built-in types are always valid.
	if qname.Namespace == types.XSDNamespace {
		if types.GetBuiltin(types.TypeName(qname.Local)) == nil {
			return fmt.Errorf("type '%s' not found in XSD namespace", qname.Local)
		}
		return nil
	}

	// check if type exists in schema.
	if _, exists := schema.TypeDefs[qname]; !exists {
		return fmt.Errorf("type %s not found", qname)
	}

	return nil
}

func validateImportForNamespace(schema *parser.Schema, contextNamespace, referenceNamespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == types.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if referenceNamespace.IsEmpty() {
		if contextNamespace.IsEmpty() {
			return nil
		}
		if imports, ok := schema.ImportedNamespaces[contextNamespace]; ok && imports[types.NamespaceEmpty] {
			return nil
		}
		return fmt.Errorf("namespace %s not imported for %s", referenceNamespace, contextNamespace)
	}
	if referenceNamespace == contextNamespace {
		return nil
	}
	if imports, ok := schema.ImportedNamespaces[contextNamespace]; ok && imports[referenceNamespace] {
		return nil
	}
	return fmt.Errorf("namespace %s not imported for %s", referenceNamespace, contextNamespace)
}

func validateImportForNamespaceAtLocation(schema *parser.Schema, location string, referenceNamespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace == types.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if location == "" || schema.ImportContexts == nil {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	ctx, ok := schema.ImportContexts[location]
	if !ok {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	if referenceNamespace.IsEmpty() {
		if ctx.TargetNamespace.IsEmpty() {
			return nil
		}
		if ctx.Imports != nil && ctx.Imports[types.NamespaceEmpty] {
			return nil
		}
		return fmt.Errorf("namespace %s must be imported by schema %s", referenceNamespace, parser.ImportContextLocation(location))
	}
	if referenceNamespace == ctx.TargetNamespace {
		return nil
	}
	if ctx.Imports != nil && ctx.Imports[referenceNamespace] {
		return nil
	}
	return fmt.Errorf("namespace %s must be imported by schema %s", referenceNamespace, parser.ImportContextLocation(location))
}

// validateSimpleTypeFinalRestriction checks if a simple type's final attribute blocks restriction derivation.
func validateSimpleTypeFinalRestriction(schema *parser.Schema, baseQName types.QName) error {
	return validateSimpleTypeFinal(schema, baseQName, types.DerivationRestriction,
		"cannot derive by restriction from type '%s' which is final for restriction")
}

// validateSimpleTypeFinalList checks if a simple type's final attribute blocks list derivation.
func validateSimpleTypeFinalList(schema *parser.Schema, itemTypeQName types.QName) error {
	return validateSimpleTypeFinal(schema, itemTypeQName, types.DerivationList,
		"cannot use type '%s' as list item type because it is final for list")
}

// validateSimpleTypeFinalUnion checks if a simple type's final attribute blocks union derivation.
func validateSimpleTypeFinalUnion(schema *parser.Schema, memberTypeQName types.QName) error {
	return validateSimpleTypeFinal(schema, memberTypeQName, types.DerivationUnion,
		"cannot use type '%s' as union member type because it is final for union")
}

func validateSimpleTypeFinal(schema *parser.Schema, qname types.QName, method types.DerivationMethod, errFmt string) error {
	if qname.IsZero() {
		return nil
	}

	// built-in types don't have final attribute.
	if qname.Namespace == types.XSDNamespace {
		return nil
	}

	typ, exists := schema.TypeDefs[qname]
	if !exists {
		return nil // type not found - already validated elsewhere.
	}

	if st, ok := types.AsSimpleType(typ); ok {
		if st.Final.Has(method) {
			return fmt.Errorf(errFmt, qname)
		}
	}

	return nil
}
