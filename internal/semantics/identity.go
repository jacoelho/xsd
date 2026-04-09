package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

// validateIdentityConstraint validates an identity constraint (key, keyref, unique).
func validateIdentityConstraint(constraint *model.IdentityConstraint) error {
	// per XSD spec section 3.11.1, identity constraint name must be an NCName
	if !model.IsValidNCName(constraint.Name) {
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
