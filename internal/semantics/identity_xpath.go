package semantics

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
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

	// Keep selector-specific diagnostics while using the shared parser policy.
	if strings.Contains(expr, "/text()") || strings.HasSuffix(expr, "text()") {
		return fmt.Errorf("selector xpath cannot select text nodes: %s", expr)
	}
	if strings.Contains(expr, "..") || strings.Contains(expr, "parent::") {
		return fmt.Errorf("selector xpath cannot use parent navigation: %s", expr)
	}
	if strings.Contains(expr, "attribute::") {
		return fmt.Errorf("selector xpath cannot use axis 'attribute::': %s", expr)
	}

	_, err := runtime.Parse(expr, nsContext, runtime.AttributesDisallowed)
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
	_, err := runtime.Parse(expr, nsContext, runtime.AttributesAllowed)
	if err == nil {
		return nil
	}
	return normalizeFieldXPathError(expr, err)
}

func parseXPathExpression(expr string, nsContext map[string]string, policy runtime.AttributePolicy) (runtime.Expression, error) {
	parsed, err := runtime.Parse(expr, nsContext, policy)
	if err != nil {
		return runtime.Expression{}, err
	}
	if len(parsed.Paths) == 0 {
		return runtime.Expression{}, fmt.Errorf("xpath contains no paths")
	}
	return parsed, nil
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
	}
	return err
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
	}
	return err
}

func disallowedAxisFromError(msg string) string {
	idx := strings.Index(msg, "'")
	if idx < 0 || idx+1 >= len(msg) {
		return ""
	}
	rest := msg[idx+1:]
	before, _, ok := strings.Cut(rest, "'")
	if !ok {
		return ""
	}
	return before
}
