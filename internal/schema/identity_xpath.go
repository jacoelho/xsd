package schema

import (
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// identityNameResolver maps parsed identity XPath QName tokens through the
// schema namespace context and runtime name table.
type identityNameResolver interface {
	ResolveIdentityQName(parts qNameParts) (QName, error)
	ResolveIdentityWildcardNamespace(prefix string) (namespaceID, error)
}

// admitIdentityXPath checks value, the xpath of the selector or field n: the
// path grammar and the prefix binding of each name test, branch by branch from
// the left, so the first violating branch is the one reported. It is the only
// owner of those diagnostics. The check builds no paths: compilation parses
// the admitted value again to intern its name tests, which keeps a successful
// compile from allocating the paths twice, and a failure there is an
// invariant.
func admitIdentityXPath(n *schemaSyntaxNode, value string) error {
	check := identityXPathParser{resolver: syntaxIdentityNameResolver{node: n}}
	var err error
	switch n.Name.Local {
	case vocab.XSDElemSelector:
		_, err = check.selectorPaths(value)
	case vocab.XSDElemField:
		_, err = check.fieldPaths(value)
	default:
		err = xsderrors.InternalInvariant("identity XPath admission of xs:" + diagnostictext.Quote(n.Name.Local))
	}
	return withSchemaCompileLocation(n, err)
}

// syntaxIdentityNameResolver checks the prefixes of name tests against the
// namespace scope of a start tag. It resolves no name: the names it returns
// are zero.
type syntaxIdentityNameResolver struct {
	node *schemaSyntaxNode
}

func (r syntaxIdentityNameResolver) ResolveIdentityQName(parts qNameParts) (QName, error) {
	if parts.Prefixed {
		if _, ok := r.node.NS.Lookup(parts.Prefix); !ok {
			return QName{}, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(parts.Prefix))
		}
	}
	return QName{}, nil
}

func (r syntaxIdentityNameResolver) ResolveIdentityWildcardNamespace(prefix string) (namespaceID, error) {
	if _, ok := r.node.NS.Lookup(prefix); !ok {
		return 0, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return 0, nil
}

// identityXPathParser parses the paths of a selector or field. A parser
// without build reports the same violations, resolves no name and builds no
// paths, so a valid path allocates nothing.
type identityXPathParser struct {
	resolver identityNameResolver
	build    bool
}

// parseIdentityPaths parses selector XPath branches for an identity constraint.
func parseIdentityPaths(xpath string, resolver identityNameResolver) ([]identityPath, error) {
	return identityXPathParser{resolver: resolver, build: true}.selectorPaths(xpath)
}

// parseIdentityFieldPaths parses field XPath branches for an identity constraint.
func parseIdentityFieldPaths(xpath string, resolver identityNameResolver) ([]identityFieldPath, error) {
	return identityXPathParser{resolver: resolver, build: true}.fieldPaths(xpath)
}

func (p identityXPathParser) selectorPaths(xpath string) ([]identityPath, error) {
	build := p.build
	var out []identityPath
	if build {
		out = make([]identityPath, 0, strings.Count(xpath, "|")+1)
	}
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
		steps, err := p.steps(part)
		if err != nil {
			return nil, err
		}
		if build {
			out = append(out, identityPath{Descendant: desc, Steps: steps})
		}
	}
	return out, nil
}

func (p identityXPathParser) fieldPaths(xpath string) ([]identityFieldPath, error) {
	build := p.build
	var out []identityFieldPath
	if build {
		out = make([]identityFieldPath, 0, strings.Count(xpath, "|")+1)
	}
	for part := range strings.SplitSeq(xpath, "|") {
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
		}
		path, err := p.fieldPathBranch(part)
		if err != nil {
			return nil, err
		}
		if build {
			out = append(out, path)
		}
	}
	return out, nil
}

func (p identityXPathParser) fieldPathBranch(part string) (identityFieldPath, error) {
	desc := false
	if rest, ok := parseIdentityDescendantPrefix(part); ok {
		desc = true
		part = rest
	}
	if part == "" {
		return identityFieldPath{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
	}
	attribute, err := parseIdentityFieldAttribute(part, p.resolver)
	if err != nil {
		return identityFieldPath{}, err
	}
	part = attribute.elementPath
	var steps []IdentityStep
	if part != "" {
		steps, err = p.steps(part)
		if err != nil {
			return identityFieldPath{}, err
		}
	}
	return identityFieldPath{
		Descendant:       desc,
		Attr:             attribute.present,
		AttrWildcard:     attribute.name.wildcard,
		AttrNamespaceSet: attribute.name.namespaceSet,
		AttrNamespace:    attribute.name.namespace,
		Steps:            steps,
		Attribute:        attribute.name.name,
	}, nil
}

type identityFieldAttribute struct {
	elementPath string
	name        identityNameTest
	present     bool
}

func parseIdentityFieldAttribute(part string, resolver identityNameResolver) (identityFieldAttribute, error) {
	if strings.HasPrefix(part, "/") {
		return identityFieldAttribute{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+diagnostictext.Quote(part))
	}
	elementPath, step := splitIdentityLastStep(part)
	if name, ok := strings.CutPrefix(step, "@"); ok {
		if name == "" {
			return identityFieldAttribute{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+diagnostictext.Quote(part))
		}
		attrName, err := parseIdentityNameTestParts(name, resolver)
		return identityFieldAttribute{elementPath: elementPath, name: attrName, present: true}, err
	}
	name, ok := parseIdentityAxisStep(step, "attribute")
	if ok && name == "" {
		return identityFieldAttribute{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+diagnostictext.Quote(part))
	}
	if !ok {
		return identityFieldAttribute{elementPath: part, name: identityNameTest{name: noQName()}}, nil
	}
	attrName, err := parseIdentityNameTestParts(name, resolver)
	return identityFieldAttribute{elementPath: elementPath, name: attrName, present: true}, err
}

func splitIdentityLastStep(path string) (elementPath, lastStep string) {
	elementPath, lastStep, found := strings.CutLast(path, "/")
	if !found {
		return "", path
	}
	return elementPath, lastStep
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

func parseIdentityNameTest(lexical string, resolver identityNameResolver) (IdentityStep, error) {
	parsed, err := parseIdentityNameTestParts(lexical, resolver)
	if err != nil {
		return IdentityStep{}, err
	}
	return IdentityStep{
		Name:         parsed.name,
		Namespace:    parsed.namespace,
		Wildcard:     parsed.wildcard,
		NamespaceSet: parsed.namespaceSet,
	}, nil
}

type identityNameTest struct {
	name         QName
	namespace    namespaceID
	wildcard     bool
	namespaceSet bool
}

func parseIdentityNameTestParts(lexical string, resolver identityNameResolver) (identityNameTest, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	if lexical == "*" {
		return identityNameTest{name: noQName(), wildcard: true}, nil
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
		return identityNameTest{name: noQName(), wildcard: true, namespaceSet: true, namespace: nsID}, nil
	}
	q, err := parseIdentityQName(lexical, resolver)
	if err != nil {
		return identityNameTest{}, err
	}
	return identityNameTest{name: q}, nil
}

func (p identityXPathParser) steps(path string) ([]IdentityStep, error) {
	if err := validateIdentityElementPath(path); err != nil {
		return nil, err
	}
	var steps []IdentityStep
	if p.build {
		steps = make([]IdentityStep, 0, strings.Count(path, "/")+1)
	}
	for part := range strings.SplitSeq(path, "/") {
		step, present, err := parseIdentityElementStep(part, p.resolver)
		if err != nil {
			return nil, err
		}
		if present && p.build {
			steps = append(steps, step)
		}
	}
	return steps, nil
}

func validateIdentityElementPath(path string) error {
	if strings.Contains(path, "@") {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath "+diagnostictext.Quote(path))
	}
	if strings.ContainsAny(path, "[]()") || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath "+diagnostictext.Quote(path))
	}
	return nil
}

func parseIdentityElementStep(part string, resolver identityNameResolver) (IdentityStep, bool, error) {
	part = lex.TrimXMLWhitespaceString(part)
	if part == "" {
		return IdentityStep{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step")
	}
	if strings.Contains(part, "::") {
		name, err := parseIdentityChildAxisStep(part)
		if err != nil {
			return IdentityStep{}, false, err
		}
		part = name
	}
	if part == "." {
		return IdentityStep{}, false, nil
	}
	step, err := parseIdentityNameTest(part, resolver)
	return step, true, err
}

func parseIdentityChildAxisStep(part string) (string, error) {
	name, ok := parseIdentityAxisStep(part, "child")
	if !ok {
		return "", xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step "+diagnostictext.Quote(part))
	}
	if name == "" || name == "." {
		return "", xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step child::")
	}
	return name, nil
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

func parseIdentityQName(lexical string, resolver identityNameResolver) (QName, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return QName{}, err
	}
	if resolver == nil {
		return QName{}, xsderrors.InternalInvariant("identity XPath parser requires name resolver")
	}
	return resolver.ResolveIdentityQName(parts)
}

func parseIdentityQNamePrefixWildcard(lexical string) (string, bool, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok || local != "*" {
		return "", false, nil
	}
	if prefix == "" || !lex.IsNCName(prefix) {
		return "", true, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, invalidQNameMessagePrefix+diagnostictext.Quote(lexical))
	}
	return prefix, true, nil
}
