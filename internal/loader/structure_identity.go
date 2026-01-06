package loader

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSelectorXPath validates that a selector XPath selects element nodes
// Selectors cannot select attributes or text nodes per XSD 1.0 spec
func validateSelectorXPath(xpath string) error {
	xpath = strings.TrimSpace(xpath)

	if xpath == "" {
		return fmt.Errorf("selector xpath cannot be empty")
	}

	// Selector cannot select text nodes
	if strings.Contains(xpath, "/text()") || strings.HasSuffix(xpath, "text()") {
		return fmt.Errorf("selector xpath cannot select text nodes: %s", xpath)
	}

	if strings.Contains(xpath, "(") || strings.Contains(xpath, ")") {
		return fmt.Errorf("selector xpath cannot use functions or parentheses: %s", xpath)
	}

	// Selector cannot select attributes (@ is not allowed anywhere in selector XPath per BNF grammar)
	if strings.Contains(xpath, "@") {
		return fmt.Errorf("selector xpath cannot select attributes: %s", xpath)
	}

	// Selector cannot use parent navigation (upward navigation not allowed)
	if strings.Contains(xpath, "..") || strings.Contains(xpath, "parent::") {
		return fmt.Errorf("selector xpath cannot use parent navigation: %s", xpath)
	}

	// Note: attribute:: and namespace:: axes are checked in validateSelectorXPathRestrictions()
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

	// Fields can only use child and attribute axes (abbreviated forms allowed).
	disallowedAxes := []string{
		"parent::", "ancestor::", "ancestor-or-self::",
		"following::", "following-sibling::",
		"preceding::", "preceding-sibling::",
		"namespace::",
	}

	for _, axis := range disallowedAxes {
		if strings.Contains(axisCheck, axis) {
			return fmt.Errorf("field xpath cannot use axis '%s': %s", axis, xpath)
		}
	}

	// Validate allowed axes only
	// Allowed: child::, attribute::, or abbreviated (@, .//)
	hasAllowedAxis := false
	allowedPatterns := []string{
		"child::", "attribute::",
		"@", // Attribute abbreviation
	}

	// If starts with @, it's an attribute (allowed)
	if strings.HasPrefix(xpath, "@") {
		hasAllowedAxis = true
	} else {
		for _, pattern := range allowedPatterns {
			if strings.Contains(axisCheck, pattern) {
				hasAllowedAxis = true
				break
			}
		}
		// If no axis specified, default is child (allowed)
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

	// Selector XPath must be a relative path expression.
	if strings.HasPrefix(xpath, "/") {
		return fmt.Errorf("selector xpath must be a relative path: %s", xpath)
	}

	// Selectors can use wildcards, but check for disallowed axes
	disallowedAxes := []string{
		"parent::", "ancestor::", "ancestor-or-self::",
		"following::", "following-sibling::",
		"preceding::", "preceding-sibling::",
		"namespace::", "attribute::",
	}

	for _, axis := range disallowedAxes {
		if strings.Contains(xpath, axis) {
			return fmt.Errorf("selector xpath cannot use axis '%s': %s", axis, xpath)
		}
	}

	return nil
}

// validateIdentityConstraint validates an identity constraint (key, keyref, unique)
func validateIdentityConstraint(schema *schema.Schema, constraint *types.IdentityConstraint, decl *types.ElementDecl) error {
	// Note: Identity constraints can be placed on elements with either simple or complex types.
	// For simple types, the selector/field XPath expressions typically target "." (the element itself).
	// The XSD spec does not restrict identity constraints to complex types only.

	// Per XSD spec section 3.11.1, identity constraint name must be an NCName
	if !isValidNCName(constraint.Name) {
		return fmt.Errorf("identity constraint name '%s' must be a valid NCName (no colons)", constraint.Name)
	}

	// Per XSD spec, 'refer' attribute is only allowed on keyref constraints
	// If present on unique or key, the schema is invalid
	if constraint.Type != types.KeyRefConstraint && !constraint.ReferQName.IsZero() {
		return fmt.Errorf("'refer' attribute is only allowed on keyref constraints, not on %s", constraint.Type)
	}

	// Selector must be present and non-empty
	if constraint.Selector.XPath == "" {
		return fmt.Errorf("identity constraint selector xpath is required")
	}

	// Validate selector selects elements (not attributes or text)
	if err := validateSelectorXPath(constraint.Selector.XPath); err != nil {
		return fmt.Errorf("identity constraint selector: %w", err)
	}

	if err := validateSelectorXPathRestrictions(constraint.Selector.XPath); err != nil {
		return fmt.Errorf("identity constraint selector: %w", err)
	}
	if err := validateRestrictedSelectorXPathGrammar(constraint.Selector.XPath, constraint.NamespaceContext); err != nil {
		return fmt.Errorf("identity constraint selector: %w", err)
	}

	// At least one field must be present
	if len(constraint.Fields) == 0 {
		return fmt.Errorf("identity constraint must have at least one field")
	}

	// All fields must have non-empty xpath
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

	// Keyref must have a refer attribute
	if constraint.Type == types.KeyRefConstraint {
		if constraint.ReferQName.IsZero() {
			return fmt.Errorf("keyref constraint must have a refer attribute")
		}
		// Refer attribute is a QName (can have namespace prefix) - validation happens during resolution
	}

	return nil
}

type xpathAttributePolicy int

const (
	xpathAttributesDisallowed xpathAttributePolicy = iota
	xpathAttributesAllowed
)

func validateRestrictedSelectorXPathGrammar(xpath string, nsContext map[string]string) error {
	return validateRestrictedXPathGrammar(xpath, nsContext, xpathAttributesDisallowed)
}

func validateRestrictedFieldXPathGrammar(xpath string, nsContext map[string]string) error {
	return validateRestrictedXPathGrammar(xpath, nsContext, xpathAttributesAllowed)
}

func validateRestrictedXPathGrammar(xpath string, nsContext map[string]string, policy xpathAttributePolicy) error {
	xpath = strings.TrimSpace(xpath)
	if xpath == "" {
		return fmt.Errorf("xpath cannot be empty")
	}

	parts := strings.SplitSeq(xpath, "|")
	for part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return fmt.Errorf("xpath contains empty union branch: %s", xpath)
		}
		parser := xpathParser{
			input:     part,
			nsContext: nsContext,
			policy:    policy,
		}
		if err := parser.parsePath(); err != nil {
			return err
		}
	}

	return nil
}

type xpathParser struct {
	input     string
	pos       int
	nsContext map[string]string
	policy    xpathAttributePolicy
}

func (p *xpathParser) parsePath() error {
	p.skipSpace()
	if p.atEnd() {
		return fmt.Errorf("xpath must contain at least one step")
	}

	var attrStep bool
	if p.consumeDotSlashSlashPrefix() {
		parsedAttrStep, err := p.parseStep()
		if err != nil {
			return err
		}
		attrStep = parsedAttrStep
	} else {
		parsedAttrStep, err := p.parseStep()
		if err != nil {
			return err
		}
		attrStep = parsedAttrStep
	}

	for {
		p.skipSpace()
		if attrStep {
			if p.peekDoubleSlash() || p.peekSlash() {
				return fmt.Errorf("xpath attribute step must be final: %s", p.input)
			}
		}
		if p.consumeDoubleSlash() {
			return fmt.Errorf("xpath cannot use '//' in restricted xpath: %s", p.input)
		}
		if !p.consumeSlash() {
			break
		}

		parsedAttrStep, err := p.parseStep()
		if err != nil {
			return err
		}
		attrStep = parsedAttrStep
	}

	p.skipSpace()
	if !p.atEnd() {
		return fmt.Errorf("xpath has invalid trailing content: %s", p.input)
	}

	return nil
}

func (p *xpathParser) parseStep() (bool, error) {
	p.skipSpace()
	if p.atEnd() {
		return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
	}
	if p.peekDoubleSlash() || p.peekSlash() {
		return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
	}

	if p.consumeAt() {
		if p.policy == xpathAttributesDisallowed {
			return false, fmt.Errorf("xpath cannot select attributes: %s", p.input)
		}
		node := p.readToken()
		if node == "" {
			return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
		}
		if err := validateXPathNodeTest(node, p.nsContext); err != nil {
			return false, err
		}
		if node == "." {
			return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
		}
		return true, nil
	}

	token := p.readToken()
	if token == "" {
		return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
	}

	axis, node, hasAxis := splitAxisToken(token)
	if hasAxis {
		if axis == "" {
			return false, fmt.Errorf("xpath step has invalid axis: %s", p.input)
		}
		if node == "" {
			node = p.readToken()
			if node == "" {
				return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
			}
		}
		return p.parseAxisStep(axis, node)
	}

	p.skipSpace()
	if p.consumeDoubleColon() {
		node = p.readToken()
		if node == "" {
			return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
		}
		return p.parseAxisStep(token, node)
	}

	if err := validateXPathNodeTest(token, p.nsContext); err != nil {
		return false, err
	}
	return false, nil
}

func splitAxisToken(token string) (axis, node string, ok bool) {
	before, after, ok0 := strings.Cut(token, "::")
	if !ok0 {
		return "", "", false
	}
	axis = strings.TrimSpace(before)
	node = strings.TrimSpace(after)
	return axis, node, true
}

func (p *xpathParser) parseAxisStep(axis, node string) (bool, error) {
	if node == "." {
		return false, fmt.Errorf("xpath step is missing a node test: %s", p.input)
	}
	if strings.Contains(node, "::") {
		return false, fmt.Errorf("xpath step has invalid axis: %s", p.input)
	}

	switch axis {
	case "child":
		if err := validateXPathNodeTest(node, p.nsContext); err != nil {
			return false, err
		}
		return false, nil
	case "attribute":
		if p.policy == xpathAttributesDisallowed {
			return false, fmt.Errorf("xpath cannot select attributes: %s", p.input)
		}
		if err := validateXPathNodeTest(node, p.nsContext); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, fmt.Errorf("xpath uses disallowed axis '%s::': %s", axis, p.input)
	}
}

func (p *xpathParser) readToken() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if isXPathWhitespace(ch) || ch == '/' || ch == '|' || ch == '@' {
			break
		}
		p.pos++
	}
	return strings.TrimSpace(p.input[start:p.pos])
}

func (p *xpathParser) consumeDotSlashSlashPrefix() bool {
	start := p.pos
	p.skipSpace()
	if p.pos >= len(p.input) || p.input[p.pos] != '.' {
		p.pos = start
		return false
	}
	p.pos++
	p.skipSpace()
	if p.pos+1 >= len(p.input) || p.input[p.pos] != '/' || p.input[p.pos+1] != '/' {
		p.pos = start
		return false
	}
	p.pos += 2
	return true
}

func (p *xpathParser) consumeDoubleSlash() bool {
	p.skipSpace()
	if p.peekDoubleSlash() {
		p.pos += 2
		return true
	}
	return false
}

func (p *xpathParser) consumeSlash() bool {
	p.skipSpace()
	if p.peekSlash() && !p.peekDoubleSlash() {
		p.pos++
		return true
	}
	return false
}

func (p *xpathParser) consumeDoubleColon() bool {
	p.skipSpace()
	if p.pos+1 < len(p.input) && p.input[p.pos] == ':' && p.input[p.pos+1] == ':' {
		p.pos += 2
		return true
	}
	return false
}

func (p *xpathParser) consumeAt() bool {
	p.skipSpace()
	if p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++
		return true
	}
	return false
}

func (p *xpathParser) peekSlash() bool {
	return p.pos < len(p.input) && p.input[p.pos] == '/'
}

func (p *xpathParser) peekDoubleSlash() bool {
	return p.pos+1 < len(p.input) && p.input[p.pos] == '/' && p.input[p.pos+1] == '/'
}

func (p *xpathParser) skipSpace() {
	for p.pos < len(p.input) && isXPathWhitespace(p.input[p.pos]) {
		p.pos++
	}
}

func (p *xpathParser) atEnd() bool {
	return p.pos >= len(p.input)
}

func validateXPathNodeTest(node string, nsContext map[string]string) error {
	if node == "*" || node == "." {
		return nil
	}
	if before, ok := strings.CutSuffix(node, ":*"); ok {
		prefix := strings.TrimSpace(before)
		if prefix == "" {
			return fmt.Errorf("xpath step has empty prefix: %s", node)
		}
		if !types.IsValidNCName(prefix) {
			return fmt.Errorf("xpath step has invalid prefix %q", node)
		}
		if nsContext != nil {
			if _, ok := nsContext[prefix]; !ok {
				return fmt.Errorf("xpath step uses undeclared prefix %q", prefix)
			}
		}
		return nil
	}
	if !types.IsValidQName(node) {
		return fmt.Errorf("xpath step has invalid QName %q", node)
	}
	if before, _, ok := strings.Cut(node, ":"); ok {
		prefix := before
		if prefix == "" {
			return fmt.Errorf("xpath step has empty prefix: %s", node)
		}
		if nsContext != nil {
			if _, ok := nsContext[prefix]; !ok {
				return fmt.Errorf("xpath step uses undeclared prefix %q", prefix)
			}
		}
	}
	return nil
}

func isXPathWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
