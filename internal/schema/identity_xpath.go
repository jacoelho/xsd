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

// admitIdentityXPath parses value, the xpath of the selector or field n, into
// values: the path grammar and the prefix binding of each name test, branch by
// branch from the left, so the first violating branch is the one reported. It
// is the only owner of those diagnostics and the only parse of the value.
// Prefixes resolve against the namespace scope of the start tag; an
// unprefixed name test has no namespace (XPath 1.0 §2.3), whatever the
// target namespace of the including context. Name tests are interned into
// names, so a MaxSchemaNames breach is reported at the start tag.
func admitIdentityXPath(n *schemaSyntaxNode, value string, names *nameTable, values *admittedValues) error {
	resolver := tagIdentityNameResolver{node: n, names: names}
	var err error
	switch n.Name.Local {
	case vocab.XSDElemSelector:
		values.selector, err = parseIdentityPaths(value, resolver)
	case vocab.XSDElemField:
		values.field, err = parseIdentityFieldPaths(value, resolver)
	default:
		err = xsderrors.InternalInvariant("identity XPath admission of xs:" + diagnostictext.Quote(n.Name.Local))
	}
	return withSchemaCompileLocation(n, err)
}

// tagIdentityNameResolver resolves the prefixes of name tests against the
// namespace scope of a start tag and interns the names into the compiler's
// name table.
type tagIdentityNameResolver struct {
	node  *schemaSyntaxNode
	names *nameTable
}

func (r tagIdentityNameResolver) ResolveIdentityQName(parts qNameParts) (QName, error) {
	ns := ""
	if parts.Prefixed {
		var ok bool
		ns, ok = r.node.NS.Lookup(parts.Prefix)
		if !ok {
			return QName{}, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(parts.Prefix))
		}
	}
	return newNameInterner(r.names).InternQName(ns, parts.Local)
}

func (r tagIdentityNameResolver) ResolveIdentityWildcardNamespace(prefix string) (namespaceID, error) {
	ns, ok := r.node.NS.Lookup(prefix)
	if !ok {
		return 0, schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return newNameInterner(r.names).InternNamespace(ns)
}

// parseIdentityPaths parses selector XPath branches for an identity constraint.
// Like every parse below, it reserves at most maxIdentityPathHint records from
// the separator count before accepting any, grows only with accepted branches
// and steps, and returns a slice of exactly their length, because the source
// record keeps it.
func parseIdentityPaths[R identityNameResolver](xpath string, resolver R) ([]identityPath, error) {
	out := make([]identityPath, 0, identityPathHint(xpath, "|"))
	for rest, more := xpath, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "|")
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity selector XPath branch is empty")
		}
		desc := false
		if rest, ok := parseIdentityDescendantPrefix(part); ok {
			desc = true
			part = rest
		}
		steps, err := parseIdentitySteps(part, resolver)
		if err != nil {
			return nil, err
		}
		out = append(out, identityPath{Descendant: desc, Steps: steps})
	}
	return exactSlice(out), nil
}

// maxIdentityPathHint bounds what a path reserves before its branches or
// steps are accepted, so a malformed path with many separators is rejected
// without allocating per separator.
const maxIdentityPathHint = 64

// identityPathHint is the number of parts xpath has when every separator
// starts an accepted part, capped at maxIdentityPathHint.
func identityPathHint(xpath, separator string) int {
	return min(strings.Count(xpath, separator)+1, maxIdentityPathHint)
}

// exactSlice returns s, or a copy of it without spare capacity when parts
// were discarded or the hint was exceeded.
func exactSlice[T any](s []T) []T {
	if len(s) == cap(s) {
		return s
	}
	return append(make([]T, 0, len(s)), s...)
}

// parseIdentityFieldPaths parses field XPath branches for an identity constraint.
func parseIdentityFieldPaths[R identityNameResolver](xpath string, resolver R) ([]identityFieldPath, error) {
	out := make([]identityFieldPath, 0, identityPathHint(xpath, "|"))
	for rest, more := xpath, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "|")
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
	return exactSlice(out), nil
}

func parseIdentityFieldPathBranch[R identityNameResolver](part string, resolver R) (identityFieldPath, error) {
	desc := false
	if rest, ok := parseIdentityDescendantPrefix(part); ok {
		desc = true
		part = rest
	}
	if part == "" {
		return identityFieldPath{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
	}
	attribute, err := parseIdentityFieldAttribute(part, resolver)
	if err != nil {
		return identityFieldPath{}, err
	}
	part = attribute.elementPath
	var steps []IdentityStep
	if part != "" {
		steps, err = parseIdentitySteps(part, resolver)
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

func parseIdentityFieldAttribute[R identityNameResolver](part string, resolver R) (identityFieldAttribute, error) {
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

func parseIdentityNameTest[R identityNameResolver](lexical string, resolver R) (IdentityStep, error) {
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

func parseIdentityNameTestParts[R identityNameResolver](lexical string, resolver R) (identityNameTest, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	if lexical == "*" {
		return identityNameTest{name: noQName(), wildcard: true}, nil
	}
	prefix, wildcard, err := parseIdentityQNamePrefixWildcard(lexical)
	if err != nil {
		return identityNameTest{}, err
	}
	if wildcard {
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

func parseIdentitySteps[R identityNameResolver](path string, resolver R) ([]IdentityStep, error) {
	if err := validateIdentityElementPath(path); err != nil {
		return nil, err
	}
	steps := make([]IdentityStep, 0, identityPathHint(path, "/"))
	for rest, more := path, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "/")
		step, present, err := parseIdentityElementStep(part, resolver)
		if err != nil {
			return nil, err
		}
		if present {
			steps = append(steps, step)
		}
	}
	return exactSlice(steps), nil
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

func parseIdentityElementStep[R identityNameResolver](part string, resolver R) (IdentityStep, bool, error) {
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

func parseIdentityQName[R identityNameResolver](lexical string, resolver R) (QName, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return QName{}, err
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
