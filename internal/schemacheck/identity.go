package schemacheck

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// validateSelectorXPath validates that a selector XPath selects element nodes
// Selectors cannot select attributes or text nodes per XSD 1.0 spec
func validateSelectorXPath(xpath string) error {
	xpath = strings.TrimSpace(xpath)

	if xpath == "" {
		return fmt.Errorf("selector xpath cannot be empty")
	}

	// selector cannot select text nodes
	if strings.Contains(xpath, "/text()") || strings.HasSuffix(xpath, "text()") {
		return fmt.Errorf("selector xpath cannot select text nodes: %s", xpath)
	}

	if strings.Contains(xpath, "(") || strings.Contains(xpath, ")") {
		return fmt.Errorf("selector xpath cannot use functions or parentheses: %s", xpath)
	}

	// selector cannot select attributes (@ is not allowed anywhere in selector XPath per BNF grammar)
	if strings.Contains(xpath, "@") {
		return fmt.Errorf("selector xpath cannot select attributes: %s", xpath)
	}

	// selector cannot use parent navigation (upward navigation not allowed)
	if strings.Contains(xpath, "..") || strings.Contains(xpath, "parent::") {
		return fmt.Errorf("selector xpath cannot use parent navigation: %s", xpath)
	}

	// note: attribute:: and namespace:: axes are checked in validateSelectorXPathRestrictions()
	// to avoid duplication and maintain separation of concerns

	return nil
}

// validateFieldXPath performs basic checks for field XPath expressions.
// Restricted XPath grammar is enforced separately.
func validateFieldXPath(xpath string) error {
	xpath = strings.TrimSpace(xpath)
	axisCheck := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, xpath)

	if xpath == "" {
		return fmt.Errorf("field xpath cannot be empty")
	}

	if strings.Contains(xpath, "(") || strings.Contains(xpath, ")") {
		return fmt.Errorf("field xpath cannot use functions or parentheses: %s", xpath)
	}

	// fields can only use child/attribute axes; descendant-or-self is allowed only via ".//".
	disallowedAxes := []string{
		"parent::", "ancestor::", "ancestor-or-self::",
		"following::", "following-sibling::",
		"preceding::", "preceding-sibling::",
		"self::", "descendant::", "descendant-or-self::",
		"namespace::",
	}

	for _, axis := range disallowedAxes {
		if strings.Contains(axisCheck, axis) {
			return fmt.Errorf("field xpath cannot use axis '%s': %s", axis, xpath)
		}
	}

	// validate allowed axes only
	hasAllowedAxis := false
	allowedPatterns := []string{
		"child::", "attribute::",
		"@", // attribute abbreviation
	}

	// if starts with @, it's an attribute (allowed)
	if strings.HasPrefix(xpath, "@") {
		hasAllowedAxis = true
	} else {
		if strings.HasPrefix(axisCheck, "//") || strings.HasPrefix(axisCheck, ".//") {
			hasAllowedAxis = true
		}
		for _, pattern := range allowedPatterns {
			if strings.Contains(axisCheck, pattern) {
				hasAllowedAxis = true
				break
			}
		}
		// if no axis specified, default is child (allowed)
		if !strings.Contains(axisCheck, "::") && !strings.HasPrefix(axisCheck, "//") {
			hasAllowedAxis = true
		}
	}

	if !hasAllowedAxis && strings.Contains(axisCheck, "::") {
		return fmt.Errorf("field xpath uses disallowed axis: %s", xpath)
	}

	return nil
}

// validateSelectorXPathRestrictions validates selector XPath restrictions
// Selectors can use wildcards but are still restricted to certain axes
// Per XSD spec section 3.11.4.2, selector XPath must be a restricted subset
func validateSelectorXPathRestrictions(xpath string) error {
	xpath = strings.TrimSpace(xpath)

	// selector XPath must be a relative path expression.
	if strings.HasPrefix(xpath, "/") {
		return fmt.Errorf("selector xpath must be a relative path: %s", xpath)
	}

	// selectors can use wildcards, but check for disallowed axes
	disallowedAxes := []string{
		"parent::", "ancestor::", "ancestor-or-self::",
		"following::", "following-sibling::",
		"preceding::", "preceding-sibling::",
		"namespace::", "attribute::",
		"self::", "descendant::", "descendant-or-self::",
	}

	for _, axis := range disallowedAxes {
		if strings.Contains(xpath, axis) {
			return fmt.Errorf("selector xpath cannot use axis '%s': %s", axis, xpath)
		}
	}

	return nil
}

// validateIdentityConstraint validates an identity constraint (key, keyref, unique)
func validateIdentityConstraint(schema *parser.Schema, constraint *types.IdentityConstraint, decl *types.ElementDecl) error {
	// note: Identity constraints can be placed on elements with either simple or complex types.
	// for simple types, the selector/field XPath expressions typically target "." (the element itself).
	// the XSD spec does not restrict identity constraints to complex types only.

	// per XSD spec section 3.11.1, identity constraint name must be an NCName
	if !isValidNCName(constraint.Name) {
		return fmt.Errorf("identity constraint name '%s' must be a valid NCName (no colons)", constraint.Name)
	}

	// per XSD spec, 'refer' attribute is only allowed on keyref constraints
	// if present on unique or key, the schema is invalid
	if constraint.Type != types.KeyRefConstraint && !constraint.ReferQName.IsZero() {
		return fmt.Errorf("'refer' attribute is only allowed on keyref constraints, not on %s", constraint.Type)
	}

	// selector must be present and non-empty
	if constraint.Selector.XPath == "" {
		return fmt.Errorf("identity constraint selector xpath is required")
	}

	// validate selector selects elements (not attributes or text)
	if err := validateSelectorXPath(constraint.Selector.XPath); err != nil {
		return fmt.Errorf("identity constraint selector: %w", err)
	}

	if err := validateSelectorXPathRestrictions(constraint.Selector.XPath); err != nil {
		return fmt.Errorf("identity constraint selector: %w", err)
	}
	if err := validateRestrictedSelectorXPathGrammar(constraint.Selector.XPath, constraint.NamespaceContext); err != nil {
		return fmt.Errorf("identity constraint selector: %w", err)
	}

	// at least one field must be present
	if len(constraint.Fields) == 0 {
		return fmt.Errorf("identity constraint must have at least one field")
	}

	// all fields must have non-empty xpath
	for i, field := range constraint.Fields {
		if field.XPath == "" {
			return fmt.Errorf("identity constraint field %d xpath is required", i+1)
		}

		if err := validateFieldXPath(field.XPath); err != nil {
			return fmt.Errorf("identity constraint field %d: %w", i+1, err)
		}
		if err := validateRestrictedFieldXPathGrammar(field.XPath, constraint.NamespaceContext); err != nil {
			return fmt.Errorf("identity constraint field %d: %w", i+1, err)
		}
	}

	// keyref must have a refer attribute
	if constraint.Type == types.KeyRefConstraint {
		if constraint.ReferQName.IsZero() {
			return fmt.Errorf("keyref constraint must have a refer attribute")
		}
		// refer attribute is a QName (can have namespace prefix) - validation happens during resolution
	}

	return nil
}

type xpathAttributePolicy int

const (
	xpathAttributesDisallowed xpathAttributePolicy = iota
	xpathAttributesAllowed
)

func validateRestrictedSelectorXPathGrammar(expr string, nsContext map[string]string) error {
	return validateRestrictedXPathGrammar(expr, nsContext, xpathAttributesDisallowed)
}

func validateRestrictedFieldXPathGrammar(expr string, nsContext map[string]string) error {
	return validateRestrictedXPathGrammar(expr, nsContext, xpathAttributesAllowed)
}

func validateRestrictedXPathGrammar(expr string, nsContext map[string]string, policy xpathAttributePolicy) error {
	allowAttributes := policy == xpathAttributesAllowed
	_, err := xpath.Parse(expr, nsContext, allowAttributes)
	return err
}
