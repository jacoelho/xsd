package semantics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

func resolveElementType(schema *parser.Schema, elementDecl *model.ElementDecl) (model.Type, error) {
	if elementDecl == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	elementType := parser.ResolveTypeReference(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	return elementType, nil
}

func resolveElementComplexType(schema *parser.Schema, elementDecl *model.ElementDecl) (*model.ComplexType, error) {
	elementType, err := resolveElementType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	ct, ok := elementType.(*model.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}
	return ct, nil
}

func resolveAttributeType(schema *parser.Schema, typ model.Type, message string, test runtime.NodeTest) (model.Type, error) {
	resolvedType := parser.ResolveTypeReference(schema, typ)
	if resolvedType == nil {
		return nil, fmt.Errorf(message, formatNodeTest(test))
	}
	return resolvedType, nil
}

func findAttributeType(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (model.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	ct, err := resolveElementComplexType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	for _, attrUse := range ct.Attributes() {
		if nodeTestMatchesQName(test, attrUse.Name) {
			resolvedType, err := resolveAttributeType(schema, attrUse.Type, "cannot resolve attribute type for '%s'", test)
			if err != nil {
				return nil, err
			}
			return resolvedType, nil
		}
	}
	for _, attrGroupQName := range ct.AttrGroups {
		if attrGroup, ok := schema.AttributeGroups[attrGroupQName]; ok {
			for _, attr := range attrGroup.Attributes {
				if nodeTestMatchesQName(test, attr.Name) {
					resolvedType, err := resolveAttributeType(schema, attr.Type, "cannot resolve attribute type for '%s' in attribute group", test)
					if err != nil {
						return nil, err
					}
					return resolvedType, nil
				}
			}
			for _, nestedAttrGroupQName := range attrGroup.AttrGroups {
				if nestedAttrGroup, ok := schema.AttributeGroups[nestedAttrGroupQName]; ok {
					for _, attr := range nestedAttrGroup.Attributes {
						if nodeTestMatchesQName(test, attr.Name) {
							resolvedType, err := resolveAttributeType(schema, attr.Type, "cannot resolve attribute type for '%s' in nested attribute group", test)
							if err != nil {
								return nil, err
							}
							return resolvedType, nil
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("%w: attribute '%s' not found in element type", ErrXPathUnresolvable, formatNodeTest(test))
}

func findAttributeTypeDescendant(schema *parser.Schema, elementDecl *model.ElementDecl, test runtime.NodeTest) (model.Type, error) {
	if isWildcardNodeTest(test) {
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	if elementDecl == nil {
		return nil, fmt.Errorf("cannot resolve attribute type without element declaration")
	}
	if attrType, err := findAttributeType(schema, elementDecl, test); err == nil {
		return attrType, nil
	}
	ct, err := resolveElementComplexType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	visited := map[*model.ComplexType]struct{}{ct: {}}
	return findAttributeTypeInContentDescendant(schema, ct.Content(), test, visited)
}

func findAttributeTypeInContentDescendant(schema *parser.Schema, content model.Content, test runtime.NodeTest, visited map[*model.ComplexType]struct{}) (model.Type, error) {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle != nil {
			return findAttributeTypeInParticleDescendant(schema, c.Particle, test, visited)
		}
	case *model.SimpleContent:
		return nil, fmt.Errorf("attribute '%s' not found in simple content", formatNodeTest(test))
	case *model.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if typ, err := findAttributeTypeInParticleDescendant(schema, c.Extension.Particle, test, visited); err == nil {
				return typ, nil
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return findAttributeTypeInParticleDescendant(schema, c.Restriction.Particle, test, visited)
		}
	case *model.EmptyContent:
		return nil, fmt.Errorf("attribute '%s' not found in empty content", formatNodeTest(test))
	}
	return nil, fmt.Errorf("attribute '%s' not found in content model", formatNodeTest(test))
}

func findAttributeTypeInParticleDescendant(schema *parser.Schema, particle model.Particle, test runtime.NodeTest, visited map[*model.ComplexType]struct{}) (model.Type, error) {
	switch p := particle.(type) {
	case *model.ElementDecl:
		elem := resolveElementReference(schema, p)
		if attrType, err := findAttributeType(schema, elem, test); err == nil {
			return attrType, nil
		}
		if elem.Type != nil {
			if resolvedType := parser.ResolveTypeReference(schema, elem.Type); resolvedType != nil {
				if ct, ok := resolvedType.(*model.ComplexType); ok {
					if _, seen := visited[ct]; !seen {
						visited[ct] = struct{}{}
						if typ, err := findAttributeTypeInContentDescendant(schema, ct.Content(), test, visited); err == nil {
							return typ, nil
						}
					}
				}
			}
		}
	case *model.ModelGroup:
		var unresolvedErr error
		for _, childParticle := range p.Particles {
			if typ, err := findAttributeTypeInParticleDescendant(schema, childParticle, test, visited); err == nil {
				return typ, nil
			} else if errors.Is(err, ErrXPathUnresolvable) && unresolvedErr == nil {
				unresolvedErr = err
			}
		}
		if unresolvedErr != nil {
			return nil, unresolvedErr
		}
		return nil, fmt.Errorf("attribute '%s' not found in model group", formatNodeTest(test))
	case *model.AnyElement:
		return nil, fmt.Errorf("%w: wildcard attribute", ErrXPathUnresolvable)
	}
	return nil, fmt.Errorf("attribute '%s' not found in particle", formatNodeTest(test))
}

func resolveFieldPathType(schema *parser.Schema, selectedElementDecl *model.ElementDecl, fieldPath runtime.Path) (model.Type, error) {
	if selectedElementDecl == nil {
		return nil, fmt.Errorf("cannot resolve field without selector element")
	}
	if fieldPath.Attribute != nil && isDescendantOnlySteps(fieldPath.Steps) {
		attrType, attrErr := findAttributeTypeDescendant(schema, selectedElementDecl, *fieldPath.Attribute)
		if attrErr != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", formatNodeTest(*fieldPath.Attribute), attrErr)
		}
		return attrType, nil
	}
	elementDecl, err := resolvePathElementDecl(schema, selectedElementDecl, fieldPath.Steps)
	if err != nil {
		return nil, fmt.Errorf("resolve field path: %w", err)
	}
	elementDecl = resolveElementReference(schema, elementDecl)
	if fieldPath.Attribute != nil {
		attrType, err := findAttributeType(schema, elementDecl, *fieldPath.Attribute)
		if err != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", formatNodeTest(*fieldPath.Attribute), err)
		}
		return attrType, nil
	}
	elementType, err := resolveElementType(schema, elementDecl)
	if err != nil {
		return nil, err
	}
	if elementDecl.Nillable {
		return elementType, ErrFieldSelectsNillable
	}
	if ct, ok := elementType.(*model.ComplexType); ok {
		if _, ok := ct.Content().(*model.SimpleContent); ok {
			baseType := ct.BaseType()
			if baseType != nil {
				return baseType, nil
			}
		}
		return nil, ErrFieldSelectsComplexContent
	}
	return elementType, nil
}

// ValidateIdentityConstraintResolution validates that identity-constraint
// selectors and fields can be resolved against the schema.
func ValidateIdentityConstraintResolution(sch *parser.Schema, constraint *model.IdentityConstraint, decl *model.ElementDecl) error {
	for i := range constraint.Fields {
		field := &constraint.Fields[i]
		hasUnion := strings.Contains(field.XPath, "|") || strings.Contains(constraint.Selector.XPath, "|")
		resolved, err := ResolveFieldType(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
		switch {
		case err == nil:
			field.ResolvedType = resolved
		case errors.Is(err, ErrFieldSelectsNillable):
			if resolved != nil {
				field.ResolvedType = resolved
			}
			if constraint.Type == model.KeyConstraint {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
			continue
		case errors.Is(err, ErrFieldSelectsComplexContent):
			continue
		case hasUnion:
			if !errors.Is(err, ErrXPathUnresolvable) && !errors.Is(err, ErrFieldXPathIncompatibleTypes) {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
		default:
			if !errors.Is(err, ErrXPathUnresolvable) && !errors.Is(err, ErrFieldXPathIncompatibleTypes) {
				return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
			}
		}
		if constraint.Type == model.KeyConstraint {
			if hasUnion {
				elemDecls, err := ResolveFieldElementDecls(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
				if err != nil {
					if errors.Is(err, ErrXPathUnresolvable) {
						continue
					}
					continue
				}
				for _, elemDecl := range elemDecls {
					if elemDecl != nil && elemDecl.Nillable {
						return fmt.Errorf("field %d '%s' selects nillable element '%s'", i+1, field.XPath, elemDecl.Name)
					}
				}
				continue
			}
			elemDecl, err := ResolveFieldElementDecl(sch, field, decl, constraint.Selector.XPath, constraint.NamespaceContext)
			if err == nil && elemDecl != nil && elemDecl.Nillable {
				return fmt.Errorf("field %d '%s' selects nillable element '%s'", i+1, field.XPath, elemDecl.Name)
			}
		}
	}
	return nil
}

// ValidateKeyrefConstraints validates keyref constraints against all known
// identity constraints.
func ValidateKeyrefConstraints(contextQName model.QName, constraints, allConstraints []*model.IdentityConstraint) []error {
	var errs []error
	for _, constraint := range constraints {
		if constraint.Type != model.KeyRefConstraint {
			continue
		}
		refQName := constraint.ReferQName
		if refQName.IsZero() {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' is missing refer attribute", contextQName, constraint.Name))
			continue
		}
		if refQName.Namespace != constraint.TargetNamespace {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' refers to '%s' in namespace '%s', which does not match target namespace '%s'", contextQName, constraint.Name, refQName.Local, refQName.Namespace, constraint.TargetNamespace))
			continue
		}
		var referencedConstraint *model.IdentityConstraint
		for _, other := range allConstraints {
			if other.Name == refQName.Local && other.TargetNamespace == refQName.Namespace {
				if other.Type == model.KeyConstraint || other.Type == model.UniqueConstraint {
					referencedConstraint = other
					break
				}
			}
		}
		if referencedConstraint == nil {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' references non-existent key/unique constraint '%s'", contextQName, constraint.Name, refQName.String()))
			continue
		}
		if len(constraint.Fields) != len(referencedConstraint.Fields) {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' has %d fields but referenced constraint '%s' has %d fields", contextQName, constraint.Name, len(constraint.Fields), refQName.String(), len(referencedConstraint.Fields)))
			continue
		}
		for i := 0; i < len(constraint.Fields); i++ {
			keyrefField := constraint.Fields[i]
			refField := referencedConstraint.Fields[i]
			if keyrefField.ResolvedType != nil && refField.ResolvedType != nil {
				if !FieldTypesCompatible(keyrefField.ResolvedType, refField.ResolvedType) {
					errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' field %d type '%s' is not compatible with referenced constraint '%s' field %d type '%s'", contextQName, constraint.Name, i+1, keyrefField.ResolvedType.Name(), refQName.String(), i+1, refField.ResolvedType.Name()))
				}
			}
		}
	}
	return errs
}

// ValidateIdentityConstraintUniqueness reports duplicate identity constraint
// names within the same target namespace.
func ValidateIdentityConstraintUniqueness(allConstraints []*model.IdentityConstraint) []error {
	var errs []error
	type constraintKey struct {
		name      string
		namespace model.NamespaceURI
	}
	constraintsByKey := make(map[constraintKey][]*model.IdentityConstraint)
	for _, constraint := range allConstraints {
		key := constraintKey{name: constraint.Name, namespace: constraint.TargetNamespace}
		constraintsByKey[key] = append(constraintsByKey[key], constraint)
	}
	for key, constraints := range constraintsByKey {
		if len(constraints) > 1 {
			errs = append(errs, fmt.Errorf("identity constraint name '%s' is not unique within target namespace '%s' (%d definitions)", key.name, key.namespace, len(constraints)))
		}
	}
	return errs
}
