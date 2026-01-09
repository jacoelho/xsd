package xpath

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

// Axis describes the XPath axis used in a step.
type Axis int

const (
	AxisChild Axis = iota
	AxisDescendant
	AxisDescendantOrSelf
	AxisSelf
)

// NodeTest matches element or attribute names.
type NodeTest struct {
	Any                bool
	Local              string
	Namespace          types.NamespaceURI
	NamespaceSpecified bool
}

// Step represents a single element step in a path.
type Step struct {
	Axis Axis
	Test NodeTest
}

// Path represents a compiled XPath path (with optional attribute selection).
type Path struct {
	Steps     []Step
	Attribute *NodeTest
}

// Expression represents a union of paths.
type Expression struct {
	Paths []Path
}

// Parse compiles an XPath expression into a set of paths.
func Parse(expr string, nsContext map[string]string, allowAttributes bool) (Expression, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return Expression{}, fmt.Errorf("xpath cannot be empty")
	}
	if strings.HasPrefix(expr, "/") {
		return Expression{}, fmt.Errorf("xpath must be a relative path: %s", expr)
	}
	if strings.Contains(expr, "[") || strings.Contains(expr, "]") {
		return Expression{}, fmt.Errorf("xpath cannot use predicates: %s", expr)
	}
	if strings.Contains(expr, "(") || strings.Contains(expr, ")") {
		return Expression{}, fmt.Errorf("xpath cannot use functions or parentheses: %s", expr)
	}

	parts := strings.Split(expr, "|")
	paths := make([]Path, 0, len(parts))
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return Expression{}, fmt.Errorf("xpath contains empty union branch: %s", expr)
		}
		path, err := parsePath(part, nsContext, allowAttributes)
		if err != nil {
			return Expression{}, err
		}
		paths = append(paths, path)
	}

	return Expression{Paths: paths}, nil
}

func parsePath(expr string, nsContext map[string]string, allowAttributes bool) (Path, error) {
	var path Path
	reader := &pathReader{input: expr}
	usedDescendantPrefix := reader.consumeDotSlashSlashPrefix()
	sawSuffix := false

	if usedDescendantPrefix {
		path.Steps = append(path.Steps, Step{Axis: AxisDescendantOrSelf, Test: NodeTest{Any: true}})
	}

	for {
		reader.skipSpace()
		if reader.atEnd() {
			if usedDescendantPrefix && !sawSuffix {
				return Path{}, fmt.Errorf("xpath step is missing a node test: %s", expr)
			}
			if len(path.Steps) == 0 && path.Attribute == nil {
				return Path{}, fmt.Errorf("xpath must contain at least one step: %s", expr)
			}
			return path, nil
		}

		if reader.peekSlash() {
			if len(path.Steps) == 0 && path.Attribute == nil && !usedDescendantPrefix {
				return Path{}, fmt.Errorf("xpath must be a relative path: %s", expr)
			}
			return Path{}, fmt.Errorf("xpath step is missing a node test: %s", expr)
		}

		token := reader.readToken()
		stepAxis, explicitAxis, stepToken, err := parseAxisToken(reader, token)
		if err != nil {
			return Path{}, err
		}

		addedSteps, attr, err := parseStep(stepAxis, stepToken, explicitAxis, nsContext, allowAttributes)
		if err != nil {
			return Path{}, err
		}
		sawSuffix = true
		path.Steps = append(path.Steps, addedSteps...)
		if attr != nil {
			path.Attribute = attr
		}

		reader.skipSpace()
		if reader.atEnd() {
			break
		}
		if path.Attribute != nil {
			return Path{}, fmt.Errorf("xpath attribute step must be final: %s", expr)
		}
		if reader.consumeSlash() {
			continue
		}
		if reader.peekDoubleSlash() {
			return Path{}, fmt.Errorf("xpath step has invalid axis: %s", expr)
		}
		return Path{}, fmt.Errorf("xpath has invalid trailing content: %s", expr)
	}

	return path, nil
}

func parseAxisToken(reader *pathReader, token string) (Axis, bool, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AxisChild, false, "", fmt.Errorf("xpath step is missing a node test")
	}

	if token == "@" {
		node := reader.readToken()
		if node == "" {
			return AxisChild, false, "", fmt.Errorf("xpath step is missing a node test")
		}
		return AxisChild, false, "@" + node, nil
	}

	if before, after, ok := strings.Cut(token, "::"); ok {
		name := strings.TrimSpace(before)
		if name == "" {
			return AxisChild, false, "", fmt.Errorf("xpath step has invalid axis")
		}
		explicitAxis, err := axisFromName(name)
		if err != nil {
			return AxisChild, false, "", err
		}
		node := strings.TrimSpace(after)
		if node == "" {
			node = reader.readToken()
			if node == "" {
				return AxisChild, false, "", fmt.Errorf("xpath step is missing a node test")
			}
		}
		return explicitAxis, true, node, nil
	}

	if reader.peekAxisSeparator() {
		explicitAxis, err := axisFromName(token)
		if err != nil {
			return AxisChild, false, "", err
		}
		reader.consumeAxisSeparator()
		node := reader.readToken()
		if node == "" {
			return AxisChild, false, "", fmt.Errorf("xpath step is missing a node test")
		}
		return explicitAxis, true, node, nil
	}

	return AxisChild, false, token, nil
}

func parseStep(axis Axis, token string, explicitAxis bool, nsContext map[string]string, allowAttributes bool) ([]Step, *NodeTest, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, fmt.Errorf("xpath step is missing a node test")
	}

	if axis == AxisAttribute {
		if !allowAttributes {
			return nil, nil, fmt.Errorf("xpath cannot select attributes: %s", token)
		}
		parsed, err := parseNodeTest(token, nsContext, true)
		if err != nil {
			return nil, nil, err
		}
		return nil, &parsed, nil
	}

	if token == "." {
		if axis != AxisChild || explicitAxis {
			return nil, nil, fmt.Errorf("xpath step has invalid axis")
		}
		return []Step{{Axis: AxisSelf, Test: NodeTest{Any: true}}}, nil, nil
	}

	if strings.HasPrefix(token, "@") {
		if !allowAttributes {
			return nil, nil, fmt.Errorf("xpath cannot select attributes: %s", token)
		}
		name := strings.TrimPrefix(token, "@")
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, nil, fmt.Errorf("xpath step is missing a node test: %s", token)
		}
		attr, err := parseNodeTest(name, nsContext, true)
		if err != nil {
			return nil, nil, err
		}
		return nil, &attr, nil
	}

	parsed, err := parseNodeTest(token, nsContext, false)
	if err != nil {
		return nil, nil, err
	}
	return []Step{{Axis: axis, Test: parsed}}, nil, nil
}

func parseNodeTest(token string, nsContext map[string]string, attribute bool) (NodeTest, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return NodeTest{}, fmt.Errorf("xpath step is missing a node test")
	}
	if token == "*" {
		return NodeTest{Any: true}, nil
	}

	if before, ok := strings.CutSuffix(token, ":*"); ok {
		prefix := strings.TrimSpace(before)
		if prefix == "" {
			return NodeTest{}, fmt.Errorf("xpath step has empty prefix: %s", token)
		}
		if !types.IsValidNCName(prefix) {
			return NodeTest{}, fmt.Errorf("xpath step has invalid prefix %q", token)
		}
		nsURI, ok := types.ResolveNamespace(prefix, nsContext)
		if !ok {
			return NodeTest{}, fmt.Errorf("xpath step uses undeclared prefix %q", prefix)
		}
		return NodeTest{
			Local:              "*",
			Namespace:          nsURI,
			NamespaceSpecified: true,
		}, nil
	}

	if !types.IsValidQName(token) {
		return NodeTest{}, fmt.Errorf("xpath step has invalid QName %q", token)
	}

	prefix, local, hasPrefix := types.SplitQName(token)
	if hasPrefix {
		nsURI, ok := types.ResolveNamespace(prefix, nsContext)
		if !ok {
			return NodeTest{}, fmt.Errorf("xpath step uses undeclared prefix %q", prefix)
		}
		return NodeTest{
			Local:              local,
			Namespace:          nsURI,
			NamespaceSpecified: true,
		}, nil
	}

	if attribute {
		return NodeTest{Local: local, NamespaceSpecified: true}, nil
	}
	if nsContext == nil {
		return NodeTest{Local: local}, nil
	}
	return NodeTest{Local: local, NamespaceSpecified: true}, nil
}

func axisFromName(name string) (Axis, error) {
	switch strings.TrimSpace(name) {
	case "child":
		return AxisChild, nil
	case "attribute":
		return AxisAttribute, nil
	default:
		return AxisChild, fmt.Errorf("xpath uses disallowed axis '%s::'", name)
	}
}

type pathReader struct {
	input string
	pos   int
}

func (r *pathReader) readToken() string {
	r.skipSpace()
	start := r.pos
	for r.pos < len(r.input) {
		ch := r.input[r.pos]
		if isXPathWhitespace(ch) || ch == '/' || ch == '|' {
			break
		}
		r.pos++
	}
	return strings.TrimSpace(r.input[start:r.pos])
}

func (r *pathReader) consumeSlash() bool {
	r.skipSpace()
	if r.peekSlash() && !r.peekDoubleSlash() {
		r.pos++
		return true
	}
	return false
}

func (r *pathReader) consumeDotSlashSlashPrefix() bool {
	r.skipSpace()
	start := r.pos
	if r.pos >= len(r.input) || r.input[r.pos] != '.' {
		return false
	}
	r.pos++
	r.skipSpace()
	if r.peekDoubleSlash() {
		r.pos += 2
		return true
	}
	r.pos = start
	return false
}

func (r *pathReader) peekSlash() bool {
	return r.pos < len(r.input) && r.input[r.pos] == '/'
}

func (r *pathReader) peekDoubleSlash() bool {
	return r.pos+1 < len(r.input) && r.input[r.pos] == '/' && r.input[r.pos+1] == '/'
}

func (r *pathReader) peekAxisSeparator() bool {
	r.skipSpace()
	return r.pos+1 < len(r.input) && r.input[r.pos] == ':' && r.input[r.pos+1] == ':'
}

func (r *pathReader) consumeAxisSeparator() bool {
	if r.peekAxisSeparator() {
		r.pos += 2
		return true
	}
	return false
}

func (r *pathReader) skipSpace() {
	for r.pos < len(r.input) && isXPathWhitespace(r.input[r.pos]) {
		r.pos++
	}
}

func (r *pathReader) atEnd() bool {
	r.skipSpace()
	return r.pos >= len(r.input)
}

func isXPathWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// AxisAttribute is only used internally for parsing attribute steps.
const AxisAttribute Axis = -1
