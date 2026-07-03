package compile

import (
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// IdentityNameResolver maps parsed identity XPath QName tokens through the
// schema namespace context and runtime name table.
type IdentityNameResolver interface {
	ResolveIdentityQName(prefix, local string, prefixed bool) (runtime.QName, error)
	ResolveIdentityWildcardNamespace(prefix string) (runtime.NamespaceID, error)
}

// ParseIdentityPaths parses selector XPath branches for an identity constraint.
func ParseIdentityPaths(xpath string, resolver IdentityNameResolver) ([]runtime.IdentityPath, error) {
	out := make([]runtime.IdentityPath, 0, strings.Count(xpath, "|")+1)
	for part := range strings.SplitSeq(xpath, "|") {
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity selector XPath branch is empty")
		}
		desc := false
		if rest, ok := parseIdentityDescendantPrefix(part); ok {
			desc = true
			part = rest
		}
		if part == "." && !desc {
			out = append(out, runtime.IdentityPath{Self: true})
			continue
		}
		steps, err := parseIdentitySteps(part, resolver)
		if err != nil {
			return nil, err
		}
		out = append(out, runtime.IdentityPath{Descendant: desc, Steps: steps})
	}
	return out, nil
}

// ParseIdentityFieldPaths parses field XPath branches for an identity constraint.
func ParseIdentityFieldPaths(xpath string, resolver IdentityNameResolver) ([]runtime.IdentityFieldPath, error) {
	out := make([]runtime.IdentityFieldPath, 0, strings.Count(xpath, "|")+1)
	for part := range strings.SplitSeq(xpath, "|") {
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
		}
		path, err := parseIdentityFieldPathBranch(part, resolver)
		if err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}

func parseIdentityFieldPathBranch(part string, resolver IdentityNameResolver) (runtime.IdentityFieldPath, error) {
	desc := false
	if rest, ok := parseIdentityDescendantPrefix(part); ok {
		desc = true
		part = rest
	}
	if part == "" {
		return runtime.IdentityFieldPath{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
	}
	if part == "." && !desc {
		return runtime.IdentityFieldPath{Self: true, Attribute: runtime.NoQName}, nil
	}
	part, attrName, attr, err := parseIdentityFieldAttribute(part, resolver)
	if err != nil {
		return runtime.IdentityFieldPath{}, err
	}
	var steps []runtime.IdentityStep
	if part != "" {
		steps, err = parseIdentitySteps(part, resolver)
		if err != nil {
			return runtime.IdentityFieldPath{}, err
		}
	}
	return runtime.IdentityFieldPath{
		Descendant:       desc,
		Attr:             attr,
		AttrWildcard:     attrName.wildcard,
		AttrNamespaceSet: attrName.namespaceSet,
		AttrNamespace:    attrName.namespace,
		Steps:            steps,
		Attribute:        attrName.name,
	}, nil
}

func parseIdentityFieldAttribute(part string, resolver IdentityNameResolver) (string, identityNameTest, bool, error) {
	if strings.HasPrefix(part, "/") {
		return "", identityNameTest{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+part)
	}
	elementPath, step := splitIdentityLastStep(part)
	if name, ok := strings.CutPrefix(step, "@"); ok {
		if name == "" {
			return "", identityNameTest{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+part)
		}
		attrName, err := parseIdentityNameTestParts(name, resolver)
		return elementPath, attrName, true, err
	}
	name, ok := parseIdentityAxisStep(step, "attribute")
	if ok && name == "" {
		return "", identityNameTest{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+part)
	}
	if !ok {
		return part, identityNameTest{name: runtime.NoQName}, false, nil
	}
	attrName, err := parseIdentityNameTestParts(name, resolver)
	return elementPath, attrName, true, err
}

func splitIdentityLastStep(path string) (string, string) {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "", path
	}
	return path[:idx], path[idx+1:]
}

func parseIdentityDescendantPrefix(path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, ".//"); ok {
		return lex.TrimXMLWhitespaceString(rest), true
	}
	if rest, ok := strings.CutPrefix(path, ". //"); ok {
		return lex.TrimXMLWhitespaceString(rest), true
	}
	return path, false
}

func parseIdentityNameTest(lexical string, resolver IdentityNameResolver) (runtime.IdentityStep, error) {
	parsed, err := parseIdentityNameTestParts(lexical, resolver)
	if err != nil {
		return runtime.IdentityStep{}, err
	}
	return runtime.IdentityStep{
		Name:         parsed.name,
		Namespace:    parsed.namespace,
		Wildcard:     parsed.wildcard,
		NamespaceSet: parsed.namespaceSet,
	}, nil
}

type identityNameTest struct {
	name         runtime.QName
	namespace    runtime.NamespaceID
	wildcard     bool
	namespaceSet bool
}

func parseIdentityNameTestParts(lexical string, resolver IdentityNameResolver) (identityNameTest, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	if lexical == "*" {
		return identityNameTest{name: runtime.NoQName, wildcard: true}, nil
	}
	prefix, wildcard, err := parseIdentityQNamePrefixWildcard(lexical)
	if err != nil {
		return identityNameTest{}, err
	}
	if wildcard {
		if resolver == nil {
			return identityNameTest{}, xsderrors.InternalInvariant("identity XPath parser requires name resolver")
		}
		nsID, nsErr := resolver.ResolveIdentityWildcardNamespace(prefix)
		if nsErr != nil {
			return identityNameTest{}, nsErr
		}
		return identityNameTest{name: runtime.NoQName, wildcard: true, namespaceSet: true, namespace: nsID}, nil
	}
	q, err := parseIdentityQName(lexical, resolver)
	if err != nil {
		return identityNameTest{}, err
	}
	return identityNameTest{name: q}, nil
}

func parseIdentitySteps(path string, resolver IdentityNameResolver) ([]runtime.IdentityStep, error) {
	if strings.Contains(path, "@") {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath "+path)
	}
	if strings.ContainsAny(path, "[]()") || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath "+path)
	}
	steps := make([]runtime.IdentityStep, 0, strings.Count(path, "/")+1)
	for part := range strings.SplitSeq(path, "/") {
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step")
		}
		if strings.Contains(part, "::") {
			name, ok := parseIdentityAxisStep(part, "child")
			if !ok {
				return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step "+part)
			}
			part = name
			if name == "" {
				return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step child::")
			}
		}
		if part == "." {
			continue
		}
		step, err := parseIdentityNameTest(part, resolver)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func parseIdentityAxisStep(part, axis string) (string, bool) {
	part = lex.TrimXMLWhitespaceString(part)
	rest, ok := strings.CutPrefix(part, axis)
	if !ok {
		return "", false
	}
	rest = lex.TrimXMLWhitespaceString(rest)
	rest, ok = strings.CutPrefix(rest, "::")
	if !ok {
		return "", false
	}
	return lex.TrimXMLWhitespaceString(rest), true
}

func parseIdentityQName(lexical string, resolver IdentityNameResolver) (runtime.QName, error) {
	parts, err := ParseQNameParts(lexical)
	if err != nil {
		return runtime.QName{}, err
	}
	if resolver == nil {
		return runtime.QName{}, xsderrors.InternalInvariant("identity XPath parser requires name resolver")
	}
	return resolver.ResolveIdentityQName(parts.Prefix, parts.Local, parts.Prefixed)
}

func parseIdentityQNamePrefixWildcard(lexical string) (string, bool, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok || local != "*" {
		return "", false, nil
	}
	if prefix == "" || !lex.IsNCName(prefix) {
		return "", true, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, invalidQNameMessagePrefix+lexical)
	}
	return prefix, true, nil
}
