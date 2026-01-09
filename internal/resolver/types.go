package resolver

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validation"
	xsdxml "github.com/jacoelho/xsd/internal/xml"
)

type typeReferencePolicy int

const (
	typeReferenceRequired typeReferencePolicy = iota
	typeReferenceAllowMissing
)

const noOriginLocation = ""

// validateTypeReferenceFromType validates that a type reference exists (from a Type interface).
func validateTypeReferenceFromType(schema *schema.Schema, typ types.Type, contextNamespace types.NamespaceURI) error {
	return validateTypeReferenceFromTypeWithPolicy(schema, typ, contextNamespace, noOriginLocation, typeReferenceRequired)
}

func validateTypeReferenceFromTypeAllowMissingAtLocation(schema *schema.Schema, typ types.Type, contextNamespace types.NamespaceURI, originLocation string) error {
	return validateTypeReferenceFromTypeWithPolicy(schema, typ, contextNamespace, originLocation, typeReferenceAllowMissing)
}

func validateTypeReferenceFromTypeWithPolicy(schema *schema.Schema, typ types.Type, contextNamespace types.NamespaceURI, originLocation string, policy typeReferencePolicy) error {
	visited := make(map[*types.ModelGroup]bool)
	return validateTypeReferenceFromTypeWithVisited(schema, typ, visited, policy, contextNamespace, originLocation)
}

// validateTypeReferenceFromTypeWithVisited validates type reference with cycle detection.
func validateTypeReferenceFromTypeWithVisited(schema *schema.Schema, typ types.Type, visited map[*types.ModelGroup]bool, policy typeReferencePolicy, contextNamespace types.NamespaceURI, originLocation string) error {
	allowMissing := policy == typeReferenceAllowMissing
	if typ != nil {
		qname := typ.Name()
		if !qname.IsZero() {
			if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
				return err
			}
		}
	}
	// built-in types are always valid.
	if typ.IsBuiltin() {
		return nil
	}

	// check if it's a placeholder SimpleType (has QName but not builtin and no Restriction/List/Union).
	if st, ok := typ.(*types.SimpleType); ok {
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
				if allowMissing && allowMissingTypeReference(schema, st.QName) {
					return nil
				}
				return fmt.Errorf("type %s not found", st.QName)
			}
		}
	}

	// check inline complex type's content model for references.
	if ct, ok := typ.(*types.ComplexType); ok {
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
func validateTypeReferences(schema *schema.Schema, qname types.QName, typ types.Type) error {
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
				if baseST, ok := t.ResolvedBase.(*types.SimpleType); ok && baseST.Final.Has(types.DerivationRestriction) {
					return fmt.Errorf("cannot derive by restriction from type '%s' which is final for restriction", baseST.QName)
				}
			}
		}
		if t.List != nil {
			if err := validateTypeQNameReferenceWithSchemaPolicy(schema, t.List.ItemType, qname.Namespace); err != nil {
				return fmt.Errorf("list itemType: %w", err)
			}
			// check if item type's final attribute blocks list derivation.
			if err := validateSimpleTypeFinalList(schema, t.List.ItemType); err != nil {
				return err
			}
		}
		if t.Union != nil {
			for i, memberType := range t.Union.MemberTypes {
				if err := validateTypeQNameReferenceWithSchemaPolicy(schema, memberType, qname.Namespace); err != nil {
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
func validateTypeQNameReference(schema *schema.Schema, qname types.QName, contextNamespace types.NamespaceURI) error {
	return validateTypeQNameReferenceWithPolicy(schema, qname, typeReferenceRequired, contextNamespace)
}

// validateTypeQNameReferenceAllowMissing validates a type QName reference that may be absent.
func validateTypeQNameReferenceAllowMissing(schema *schema.Schema, qname types.QName, contextNamespace types.NamespaceURI) error {
	return validateTypeQNameReferenceWithPolicy(schema, qname, typeReferenceAllowMissing, contextNamespace)
}

func validateTypeQNameReferenceWithSchemaPolicy(schema *schema.Schema, qname types.QName, contextNamespace types.NamespaceURI) error {
	if allowMissingTypeReference(schema, qname) {
		return validateTypeQNameReferenceAllowMissing(schema, qname, contextNamespace)
	}
	return validateTypeQNameReference(schema, qname, contextNamespace)
}

func validateTypeQNameReferenceWithPolicy(schema *schema.Schema, qname types.QName, policy typeReferencePolicy, contextNamespace types.NamespaceURI) error {
	allowMissing := policy == typeReferenceAllowMissing
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
		if allowMissing && allowMissingTypeReference(schema, qname) {
			return nil
		}
		return fmt.Errorf("type %s not found", qname)
	}

	return nil
}

func allowMissingTypeReference(schema *schema.Schema, qname types.QName) bool {
	if schema == nil {
		return false
	}
	return schema.TargetNamespace.IsEmpty() && qname.Namespace.IsEmpty()
}

func validateImportForNamespace(schema *schema.Schema, contextNamespace, referenceNamespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace.IsEmpty() || referenceNamespace == types.XSDNamespace || referenceNamespace == contextNamespace {
		return nil
	}
	if imports, ok := schema.ImportedNamespaces[contextNamespace]; ok && imports[referenceNamespace] {
		return nil
	}
	return fmt.Errorf("namespace %s not imported for %s", referenceNamespace, contextNamespace)
}

func validateImportForNamespaceAtLocation(schema *schema.Schema, location string, referenceNamespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if referenceNamespace.IsEmpty() || referenceNamespace == types.XSDNamespace || referenceNamespace == xsdxml.XMLNamespace {
		return nil
	}
	if location == "" || schema.ImportContexts == nil {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	ctx, ok := schema.ImportContexts[location]
	if !ok {
		return validateImportForNamespace(schema, schema.TargetNamespace, referenceNamespace)
	}
	if referenceNamespace == ctx.TargetNamespace {
		return nil
	}
	if ctx.Imports != nil && ctx.Imports[referenceNamespace] {
		return nil
	}
	return fmt.Errorf("namespace %s must be imported by schema %s", referenceNamespace, location)
}

// validateSimpleTypeFinalRestriction checks if a simple type's final attribute blocks restriction derivation.
func validateSimpleTypeFinalRestriction(schema *schema.Schema, baseQName types.QName) error {
	if baseQName.IsZero() {
		return nil
	}

	// built-in types don't have final attribute.
	if baseQName.Namespace == types.XSDNamespace {
		return nil
	}

	// look up the type.
	baseType, exists := schema.TypeDefs[baseQName]
	if !exists {
		return nil // type not found - already validated elsewhere.
	}

	// check if it's a simple type with final="restriction".
	if st, ok := baseType.(*types.SimpleType); ok {
		if st.Final.Has(types.DerivationRestriction) {
			return fmt.Errorf("cannot derive by restriction from type '%s' which is final for restriction", baseQName)
		}
	}

	return nil
}

// validateSimpleTypeFinalList checks if a simple type's final attribute blocks list derivation.
func validateSimpleTypeFinalList(schema *schema.Schema, itemTypeQName types.QName) error {
	if itemTypeQName.IsZero() {
		return nil
	}

	// built-in types don't have final attribute.
	if itemTypeQName.Namespace == types.XSDNamespace {
		return nil
	}

	// look up the type.
	itemType, exists := schema.TypeDefs[itemTypeQName]
	if !exists {
		return nil // type not found - already validated elsewhere.
	}

	// check if it's a simple type with final="list".
	if st, ok := itemType.(*types.SimpleType); ok {
		if st.Final.Has(types.DerivationList) {
			return fmt.Errorf("cannot use type '%s' as list item type because it is final for list", itemTypeQName)
		}
	}

	return nil
}

// validateSimpleTypeFinalUnion checks if a simple type's final attribute blocks union derivation.
func validateSimpleTypeFinalUnion(schema *schema.Schema, memberTypeQName types.QName) error {
	if memberTypeQName.IsZero() {
		return nil
	}

	// built-in types don't have final attribute.
	if memberTypeQName.Namespace == types.XSDNamespace {
		return nil
	}

	// look up the type.
	memberType, exists := schema.TypeDefs[memberTypeQName]
	if !exists {
		return nil // type not found - already validated elsewhere.
	}

	// check if it's a simple type with final="union".
	if st, ok := memberType.(*types.SimpleType); ok {
		if st.Final.Has(types.DerivationUnion) {
			return fmt.Errorf("cannot use type '%s' as union member type because it is final for union", memberTypeQName)
		}
	}

	return nil
}

// resolveTypeForFinalValidation resolves a type reference for substitution group final validation.
func resolveTypeForFinalValidation(schema *schema.Schema, typ types.Type) types.Type {
	return validation.ResolveTypeReference(schema, typ, true)
}
