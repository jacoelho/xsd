package semanticcheck

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/identitypath"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/qname"
)

// validateSelectorXPath validates that a selector XPath selects element nodes.
func validateSelectorXPath(expr string) error {
	return validateSelectorXPathWithContext(expr, nil)
}

func validateSelectorXPathWithContext(expr string, nsContext map[string]string) error {
	expr = model.TrimXMLWhitespace(expr)
	if expr == "" {
		return fmt.Errorf("selector xpath cannot be empty")
	}

	// keep selector-specific diagnostics while using shared parser policy.
	if strings.Contains(expr, "/text()") || strings.HasSuffix(expr, "text()") {
		return fmt.Errorf("selector xpath cannot select text nodes: %s", expr)
	}
	if strings.Contains(expr, "..") || strings.Contains(expr, "parent::") {
		return fmt.Errorf("selector xpath cannot use parent navigation: %s", expr)
	}
	if strings.Contains(expr, "attribute::") {
		return fmt.Errorf("selector xpath cannot use axis 'attribute::': %s", expr)
	}

	_, err := identitypath.ParseSelector(expr, nsContext)
	if err == nil {
		return nil
	}
	return normalizeSelectorXPathError(expr, err)
}

// validateFieldXPath performs checks for field XPath expressions.
func validateFieldXPath(expr string) error {
	return validateFieldXPathWithContext(expr, nil)
}

func validateFieldXPathWithContext(expr string, nsContext map[string]string) error {
	expr = model.TrimXMLWhitespace(expr)
	if expr == "" {
		return fmt.Errorf("field xpath cannot be empty")
	}
	_, err := identitypath.ParseField(expr, nsContext)
	if err == nil {
		return nil
	}
	return normalizeFieldXPathError(expr, err)
}

func normalizeSelectorXPathError(expr string, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "xpath cannot be empty"):
		return fmt.Errorf("selector xpath cannot be empty")
	case strings.Contains(msg, "xpath must be a relative path"):
		return fmt.Errorf("selector xpath must be a relative path: %s", expr)
	case strings.Contains(msg, "xpath cannot select attributes"):
		return fmt.Errorf("selector xpath cannot select attributes: %s", expr)
	case strings.Contains(msg, "xpath cannot use functions or parentheses"):
		return fmt.Errorf("selector xpath cannot use functions or parentheses: %s", expr)
	case strings.Contains(msg, "xpath uses disallowed axis"):
		if axis := disallowedAxisFromError(msg); axis != "" {
			return fmt.Errorf("selector xpath cannot use axis '%s': %s", axis, expr)
		}
		return fmt.Errorf("selector xpath uses disallowed axis: %s", expr)
	default:
		return err
	}
}

func normalizeFieldXPathError(expr string, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "xpath cannot be empty"):
		return fmt.Errorf("field xpath cannot be empty")
	case strings.Contains(msg, "xpath cannot use functions or parentheses"):
		return fmt.Errorf("field xpath cannot use functions or parentheses: %s", expr)
	case strings.Contains(msg, "xpath uses disallowed axis"):
		if axis := disallowedAxisFromError(msg); axis != "" {
			return fmt.Errorf("field xpath cannot use axis '%s': %s", axis, expr)
		}
		return fmt.Errorf("field xpath uses disallowed axis: %s", expr)
	default:
		return err
	}
}

func disallowedAxisFromError(msg string) string {
	idx := strings.Index(msg, "'")
	if idx < 0 || idx+1 >= len(msg) {
		return ""
	}
	rest := msg[idx+1:]
	end := strings.Index(rest, "'")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// validateIdentityConstraint validates an identity constraint (key, keyref, unique).
func validateIdentityConstraint(constraint *model.IdentityConstraint) error {
	// per XSD spec section 3.11.1, identity constraint name must be an NCName
	if !qname.IsValidNCName(constraint.Name) {
		return fmt.Errorf("identity constraint name '%s' must be a valid NCName (no colons)", constraint.Name)
	}

	// per XSD spec, 'refer' attribute is only allowed on keyref constraints
	if constraint.Type != model.KeyRefConstraint && !constraint.ReferQName.IsZero() {
		return fmt.Errorf("'refer' attribute is only allowed on keyref constraints, not on %s", constraint.Type)
	}

	// selector must be present and non-empty
	if constraint.Selector.XPath == "" {
		return fmt.Errorf("identity constraint selector xpath is required")
	}

	if err := validateSelectorXPathWithContext(constraint.Selector.XPath, constraint.NamespaceContext); err != nil {
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
		if err := validateFieldXPathWithContext(field.XPath, constraint.NamespaceContext); err != nil {
			return fmt.Errorf("identity constraint field %d: %w", i+1, err)
		}
	}

	// keyref must have a refer attribute
	if constraint.Type == model.KeyRefConstraint {
		if constraint.ReferQName.IsZero() {
			return fmt.Errorf("keyref constraint must have a refer attribute")
		}
	}

	return nil
}
