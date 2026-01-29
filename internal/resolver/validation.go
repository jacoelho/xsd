package resolver

import (
	stdErrors "errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

func validateReferences(schema *parser.Schema) []error {
	var errors []error

	elementRefsInContent := collectElementReferencesInSchema(schema)
	allConstraints := collectAllIdentityConstraints(schema)

	// per XSD spec 3.11.2: "Constraint definition identities must be unique within an XML Schema"
	// constraints are identified by (name, target namespace)
	if uniquenessErrors := validateIdentityConstraintUniqueness(schema); len(uniquenessErrors) > 0 {
		errors = append(errors, uniquenessErrors...)
	}

	errors = append(errors, validateTopLevelElementReferences(schema)...)
	errors = append(errors, validateContentElementReferences(schema, elementRefsInContent)...)
	errors = append(errors, validateElementDeclarationReferences(schema, allConstraints)...)

	if err := validateNoCyclicSubstitutionGroups(schema); err != nil {
		errors = append(errors, err)
	}

	errors = append(errors, validateLocalIdentityConstraintKeyrefs(schema, allConstraints)...)
	errors = append(errors, validateLocalIdentityConstraintResolution(schema)...)
	errors = append(errors, validateAttributeDeclarations(schema)...)
	errors = append(errors, validateTypeDefinitionReferences(schema)...)
	errors = append(errors, validateInlineTypeReferences(schema)...)
	errors = append(errors, validateComplexTypeReferences(schema)...)
	errors = append(errors, validateAttributeGroupReferencesInSchema(schema)...)
	errors = append(errors, validateLocalElementValueConstraints(schema)...)
	errors = append(errors, validateGroupReferencesInSchema(schema)...)

	if err := validateNoCyclicAttributeGroups(schema); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// ValidateReferences exposes reference validation for schema loading.
func ValidateReferences(schema *parser.Schema) []error {
	return validateReferences(schema)
}

func collectElementReferencesInSchema(schema *parser.Schema) []*types.ElementDecl {
	var elementRefsInContent []*types.ElementDecl

	for _, decl := range schema.ElementDecls {
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, collectElementReferences(ct.Content())...)
		}
	}

	for _, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			elementRefsInContent = append(elementRefsInContent, collectElementReferences(ct.Content())...)
		}
	}

	for _, group := range schema.Groups {
		for _, particle := range group.Particles {
			if elem, ok := particle.(*types.ElementDecl); ok && elem.IsReference {
				elementRefsInContent = append(elementRefsInContent, elem)
			} else if mg, ok := particle.(*types.ModelGroup); ok {
				elementRefsInContent = append(elementRefsInContent, collectElementReferencesFromParticles(mg.Particles)...)
			}
		}
	}

	return elementRefsInContent
}

func validateTopLevelElementReferences(schema *parser.Schema) []error {
	var errors []error

	for qname, decl := range schema.ElementDecls {
		if decl.IsReference {
			refDecl, exists := schema.ElementDecls[decl.Name]
			if !exists {
				errors = append(errors, fmt.Errorf("element reference %s does not exist", decl.Name))
			} else if refDecl.IsReference {
				errors = append(errors, fmt.Errorf("element reference %s points to another reference %s (circular or invalid)", qname, decl.Name))
			}
		}
	}

	return errors
}

func validateContentElementReferences(schema *parser.Schema, elementRefsInContent []*types.ElementDecl) []error {
	var errors []error

	for _, elemRef := range elementRefsInContent {
		refDecl, exists := schema.ElementDecls[elemRef.Name]
		if !exists {
			errors = append(errors, fmt.Errorf("element reference %s in content model does not exist", elemRef.Name))
		} else if refDecl.IsReference {
			errors = append(errors, fmt.Errorf("element reference %s in content model points to another reference (circular or invalid)", elemRef.Name))
		}
	}

	return errors
}

func validateElementDeclarationReferences(schema *parser.Schema, allConstraints []*types.IdentityConstraint) []error {
	var errors []error

	for qname, decl := range schema.ElementDecls {
		if decl.Type != nil {
			origin := schema.ElementOrigins[qname]
			if err := validateTypeReferenceFromTypeAllowMissingAtLocation(schema, decl.Type, qname.Namespace, origin); err != nil {
				errors = append(errors, fmt.Errorf("element %s: %w", qname, err))
			}
		}

		if err := validateElementValueConstraints(schema, decl); err != nil {
			errors = append(errors, fmt.Errorf("element %s: %w", qname, err))
		}

		if decl.SubstitutionGroup != (types.QName{}) {
			headDecl, exists := schema.ElementDecls[decl.SubstitutionGroup]
			if !exists {
				continue
			}
			if err := validateSubstitutionGroupDerivation(schema, qname, decl, headDecl); err != nil {
				errors = append(errors, err)
			}
			if err := validateSubstitutionGroupFinal(schema, qname, decl, headDecl); err != nil {
				errors = append(errors, err)
			}
		}

		if err := validateKeyrefConstraints(qname, decl.Constraints, allConstraints); err != nil {
			errors = append(errors, err...)
		}

		for _, constraint := range decl.Constraints {
			if err := validateIdentityConstraintResolution(schema, constraint, decl); err != nil {
				errors = append(errors, fmt.Errorf("element %s identity constraint '%s': %w", qname, constraint.Name, err))
			}
		}
	}

	return errors
}

func validateLocalIdentityConstraintKeyrefs(schema *parser.Schema, allConstraints []*types.IdentityConstraint) []error {
	var errors []error

	for qname, decl := range schema.ElementDecls {
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			localConstraints := collectIdentityConstraintsFromContent(ct.Content())
			if len(localConstraints) > 0 {
				if err := validateKeyrefConstraints(qname, localConstraints, allConstraints); err != nil {
					errors = append(errors, err...)
				}
			}
		}
	}
	for qname, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			localConstraints := collectIdentityConstraintsFromContent(ct.Content())
			if len(localConstraints) > 0 {
				if err := validateKeyrefConstraints(qname, localConstraints, allConstraints); err != nil {
					errors = append(errors, err...)
				}
			}
		}
	}

	return errors
}

func validateLocalIdentityConstraintResolution(schema *parser.Schema) []error {
	var errors []error

	for qname, decl := range schema.ElementDecls {
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			localConstraints := collectIdentityConstraintsFromContent(ct.Content())
			for _, constraint := range localConstraints {
				tempDecl := &types.ElementDecl{Type: ct}
				if err := validateIdentityConstraintResolution(schema, constraint, tempDecl); err != nil {
					if stdErrors.Is(err, xpath.ErrInvalidXPath) {
						continue
					}
					errors = append(errors, fmt.Errorf("element %s local identity constraint '%s': %w", qname, constraint.Name, err))
				}
			}
		}
	}
	for qname, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			localConstraints := collectIdentityConstraintsFromContent(ct.Content())
			for _, constraint := range localConstraints {
				tempDecl := &types.ElementDecl{Type: ct}
				if err := validateIdentityConstraintResolution(schema, constraint, tempDecl); err != nil {
					if stdErrors.Is(err, xpath.ErrInvalidXPath) {
						continue
					}
					errors = append(errors, fmt.Errorf("type %s local identity constraint '%s': %w", qname, constraint.Name, err))
				}
			}
		}
	}

	return errors
}

func validateAttributeDeclarations(schema *parser.Schema) []error {
	var errors []error

	// note: Attribute references are stored in complex types, not as top-level declarations
	// we validate attribute type references when validating complex types
	for qname, decl := range schema.AttributeDecls {
		if decl.Type != nil {
			if err := validateTypeReferenceFromType(schema, decl.Type, qname.Namespace); err != nil {
				errors = append(errors, fmt.Errorf("attribute %s: %w", qname, err))
			}
		}

		// validate default/fixed values against the resolved type (including facets)
		// this is done here after type resolution because during structure validation
		// the type might be a placeholder and facets might not be available
		resolvedType := resolveTypeForFinalValidation(schema, decl.Type)
		if _, ok := resolvedType.(*types.ComplexType); ok {
			errors = append(errors, fmt.Errorf("attribute %s: type must be a simple type", qname))
		}
		if decl.HasDefault {
			if err := validateDefaultOrFixedValueWithResolvedType(schema, decl.Default, resolvedType, decl.DefaultContext); err != nil {
				errors = append(errors, fmt.Errorf("attribute %s: invalid default value '%s': %w", qname, decl.Default, err))
			}
		}
		if decl.HasFixed {
			if err := validateDefaultOrFixedValueWithResolvedType(schema, decl.Fixed, resolvedType, decl.FixedContext); err != nil {
				errors = append(errors, fmt.Errorf("attribute %s: invalid fixed value '%s': %w", qname, decl.Fixed, err))
			}
		}
	}

	return errors
}

func validateTypeDefinitionReferences(schema *parser.Schema) []error {
	var errors []error

	for qname, typ := range schema.TypeDefs {
		if err := validateTypeReferences(schema, qname, typ); err != nil {
			errors = append(errors, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errors
}

func validateInlineTypeReferences(schema *parser.Schema) []error {
	var errors []error

	for qname, decl := range schema.ElementDecls {
		if decl.Type != nil && !decl.Type.IsBuiltin() {
			// skip if the type is a reference to a named type (already validated above)
			if _, exists := schema.TypeDefs[decl.Type.Name()]; !exists {
				if err := validateTypeReferences(schema, qname, decl.Type); err != nil {
					errors = append(errors, fmt.Errorf("element %s inline type: %w", qname, err))
				}
				// also validate attribute group references for inline complex types
				if ct, ok := decl.Type.(*types.ComplexType); ok {
					for _, agRef := range ct.AttrGroups {
						if err := validateAttributeGroupReference(schema, agRef, qname); err != nil {
							errors = append(errors, err)
						}
					}
					for _, attr := range ct.Attributes() {
						if attr.IsReference {
							if err := validateAttributeReference(schema, qname, attr, "element"); err != nil {
								errors = append(errors, err)
							}
						}
					}
				}
			}
		}
	}

	return errors
}

func validateComplexTypeReferences(schema *parser.Schema) []error {
	var errors []error

	for qname, typ := range schema.TypeDefs {
		ct, ok := typ.(*types.ComplexType)
		if !ok {
			continue
		}
		for _, agRef := range ct.AttrGroups {
			if err := validateAttributeGroupReference(schema, agRef, qname); err != nil {
				errors = append(errors, err)
			}
		}

		if cc, ok := ct.Content().(*types.ComplexContent); ok {
			if cc.Extension != nil {
				for _, agRef := range cc.Extension.AttrGroups {
					if err := validateAttributeGroupReference(schema, agRef, qname); err != nil {
						errors = append(errors, err)
					}
				}
			}
			if cc.Restriction != nil {
				for _, agRef := range cc.Restriction.AttrGroups {
					if err := validateAttributeGroupReference(schema, agRef, qname); err != nil {
						errors = append(errors, err)
					}
				}
			}
		}
		if sc, ok := ct.Content().(*types.SimpleContent); ok {
			if sc.Extension != nil {
				for _, agRef := range sc.Extension.AttrGroups {
					if err := validateAttributeGroupReference(schema, agRef, qname); err != nil {
						errors = append(errors, err)
					}
				}
			}
		}

		for _, attr := range ct.Attributes() {
			if attr.IsReference {
				if err := validateAttributeReference(schema, qname, attr, "type"); err != nil {
					errors = append(errors, err)
				}
			} else if attr.Type != nil {
				if err := validateTypeReferenceFromType(schema, attr.Type, qname.Namespace); err != nil {
					errors = append(errors, fmt.Errorf("type %s attribute: %w", qname, err))
				}
			}
		}

		origin := schema.TypeOrigins[qname]
		if err := validateContentReferences(schema, ct.Content(), origin); err != nil {
			errors = append(errors, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errors
}

func validateAttributeGroupReferencesInSchema(schema *parser.Schema) []error {
	var errors []error

	for qname, ag := range schema.AttributeGroups {
		for _, agRef := range ag.AttrGroups {
			if err := validateAttributeGroupReference(schema, agRef, qname); err != nil {
				errors = append(errors, err)
			}
		}

		for _, attr := range ag.Attributes {
			if attr.IsReference {
				if err := validateAttributeReference(schema, qname, attr, "attributeGroup"); err != nil {
					errors = append(errors, err)
				}
			}
		}

		for _, attr := range ag.Attributes {
			if attr.Type != nil {
				if err := validateTypeReferenceFromType(schema, attr.Type, qname.Namespace); err != nil {
					errors = append(errors, fmt.Errorf("attributeGroup %s attribute %s: %w", qname, attr.Name, err))
				}
			}
		}
	}

	return errors
}

func validateLocalElementValueConstraints(schema *parser.Schema) []error {
	var errors []error

	seenLocal := make(map[*types.ElementDecl]bool)
	validateLocals := func(ct *types.ComplexType) {
		for _, elem := range schemacheck.CollectAllElementDeclarationsFromType(schema, ct) {
			if elem == nil || elem.IsReference {
				continue
			}
			if seenLocal[elem] {
				continue
			}
			seenLocal[elem] = true
			if err := validateElementValueConstraints(schema, elem); err != nil {
				errors = append(errors, fmt.Errorf("local element %s: %w", elem.Name, err))
			}
		}
	}
	for _, decl := range schema.ElementDecls {
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}
	for _, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}

	return errors
}

func validateGroupReferencesInSchema(schema *parser.Schema) []error {
	var errors []error

	for qname, group := range schema.Groups {
		if err := validateGroupReferences(schema, qname, group); err != nil {
			errors = append(errors, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	return errors
}
