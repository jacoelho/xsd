package schema

import (
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// identityNameResolver binds the prefixes of identity XPath name tests and
// interns their names. A parse resolves every prefix of a branch before it
// interns any of the branch's names.
type identityNameResolver interface {
	ResolveIdentityPrefix(prefix string) (string, error)
	InternIdentityQName(namespace, local string) (QName, error)
	InternIdentityNamespace(namespace string) (namespaceID, error)
}

// admitIdentityXPath parses value, the xpath of the selector or field n, into
// values: the path grammar and the prefix binding of each name test, branch by
// branch from the left, so the first violating branch is the one reported. It
// is the only owner of those diagnostics and the only parse of the value.
// Prefixes resolve against the namespace scope of the start tag; an
// unprefixed name test has no namespace (XPath 1.0 §2.3), whatever the
// target namespace of the including context. The names of a branch are
// interned into names once its grammar and prefixes are accepted, so a
// MaxSchemaNames breach is reported at the start tag and never in place of the
// branch's syntax or binding error.
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

func (r tagIdentityNameResolver) ResolveIdentityPrefix(prefix string) (string, error) {
	ns, ok := r.node.NS.Lookup(prefix)
	if !ok {
		return "", schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(prefix))
	}
	return ns, nil
}

func (r tagIdentityNameResolver) InternIdentityQName(namespace, local string) (QName, error) {
	return newNameInterner(r.names).InternQName(namespace, local)
}

func (r tagIdentityNameResolver) InternIdentityNamespace(namespace string) (namespaceID, error) {
	return newNameInterner(r.names).InternNamespace(namespace)
}

// identityNameSyntax is a name test whose prefix is bound and whose names are
// not interned yet.
type identityNameSyntax struct {
	namespace string
	local     string
	kind      identityNameKind
}

type identityNameKind uint8

const (
	// identityNameAny is the name test *.
	identityNameAny identityNameKind = iota
	// identityNameNamespace is the name test prefix:*.
	identityNameNamespace
	// identityNameQName is a QName name test.
	identityNameQName
)

// identityBranchNames is the number of name tests a branch holds before its
// pending names spill from the stack.
const identityBranchNames = 8

func internIdentityName[R identityNameResolver](syntax identityNameSyntax, resolver R) (identityNameTest, error) {
	switch syntax.kind {
	case identityNameNamespace:
		ns, err := resolver.InternIdentityNamespace(syntax.namespace)
		if err != nil {
			return identityNameTest{}, err
		}
		return identityNameTest{name: noQName(), wildcard: true, namespaceSet: true, namespace: ns}, nil
	case identityNameQName:
		q, err := resolver.InternIdentityQName(syntax.namespace, syntax.local)
		if err != nil {
			return identityNameTest{}, err
		}
		return identityNameTest{name: q}, nil
	case identityNameAny:
		return identityNameTest{name: noQName(), wildcard: true}, nil
	default:
		return identityNameTest{}, xsderrors.InternalInvariant("unknown identity XPath name test kind")
	}
}

// internIdentitySteps returns the element steps of the accepted name tests
// names, in a slice of exactly their length.
func internIdentitySteps[R identityNameResolver](names []identityNameSyntax, resolver R) ([]IdentityStep, error) {
	if len(names) == 0 {
		return nil, nil
	}
	steps := make([]IdentityStep, len(names))
	for i, syntax := range names {
		test, err := internIdentityName(syntax, resolver)
		if err != nil {
			return nil, err
		}
		steps[i] = IdentityStep{Name: test.name, Namespace: test.namespace, Wildcard: test.wildcard, NamespaceSet: test.namespaceSet}
	}
	return steps, nil
}

// parseIdentityPaths parses selector XPath branches for an identity constraint.
// Like the field parse, it reserves at most maxIdentityPathHint branches from
// the separator count before accepting any, grows only with accepted branches
// and name tests, and returns slices of exactly the accepted length, because
// the source record keeps them.
func parseIdentityPaths[R identityNameResolver](xpath string, resolver R) ([]identityPath, error) {
	out := make([]identityPath, 0, identityPathHint(xpath, "|"))
	var pending [identityBranchNames]identityNameSyntax
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
		names, err := parseIdentityStepNames(part, resolver, pending[:0])
		if err != nil {
			return nil, err
		}
		steps, err := internIdentitySteps(names, resolver)
		if err != nil {
			return nil, err
		}
		out = append(out, identityPath{Descendant: desc, Steps: steps})
	}
	return exactSlice(out), nil
}

// maxIdentityPathHint bounds what a path reserves before its branches are
// accepted, so a malformed path with many separators is rejected without
// allocating per separator.
const maxIdentityPathHint = 64

// identityPathHint is the number of branches xpath has when every separator
// starts an accepted branch, capped at maxIdentityPathHint.
func identityPathHint(xpath, separator string) int {
	return min(strings.Count(xpath, separator)+1, maxIdentityPathHint)
}

// exactSlice returns s, or a copy of it without spare capacity when branches
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
	var pending [identityBranchNames]identityNameSyntax
	for rest, more := xpath, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "|")
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
		}
		path, err := parseIdentityFieldPathBranch(part, resolver, pending[:0])
		if err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return exactSlice(out), nil
}

func parseIdentityFieldPathBranch[R identityNameResolver](part string, resolver R, pending []identityNameSyntax) (identityFieldPath, error) {
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
	names := pending
	if attribute.elementPath != "" {
		names, err = parseIdentityStepNames(attribute.elementPath, resolver, pending)
		if err != nil {
			return identityFieldPath{}, err
		}
	}
	steps, err := internIdentitySteps(names, resolver)
	if err != nil {
		return identityFieldPath{}, err
	}
	name := identityNameTest{name: noQName()}
	if attribute.present {
		name, err = internIdentityName(attribute.name, resolver)
		if err != nil {
			return identityFieldPath{}, err
		}
	}
	return identityFieldPath{
		Descendant:       desc,
		Attr:             attribute.present,
		AttrWildcard:     name.wildcard,
		AttrNamespaceSet: name.namespaceSet,
		AttrNamespace:    name.namespace,
		Steps:            steps,
		Attribute:        name.name,
	}, nil
}

type identityFieldAttribute struct {
	elementPath string
	name        identityNameSyntax
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
		attrName, err := parseIdentityNameSyntax(name, resolver)
		return identityFieldAttribute{elementPath: elementPath, name: attrName, present: true}, err
	}
	name, ok := parseIdentityAxisStep(step, "attribute")
	if ok && name == "" {
		return identityFieldAttribute{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity field XPath "+diagnostictext.Quote(part))
	}
	if !ok {
		return identityFieldAttribute{elementPath: part}, nil
	}
	attrName, err := parseIdentityNameSyntax(name, resolver)
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

type identityNameTest struct {
	name         QName
	namespace    namespaceID
	wildcard     bool
	namespaceSet bool
}

func parseIdentityNameSyntax[R identityNameResolver](lexical string, resolver R) (identityNameSyntax, error) {
	lexical = lex.TrimXMLWhitespaceString(lexical)
	if lexical == "*" {
		return identityNameSyntax{kind: identityNameAny}, nil
	}
	prefix, wildcard, err := parseIdentityQNamePrefixWildcard(lexical)
	if err != nil {
		return identityNameSyntax{}, err
	}
	if wildcard {
		ns, nsErr := resolver.ResolveIdentityPrefix(prefix)
		if nsErr != nil {
			return identityNameSyntax{}, nsErr
		}
		return identityNameSyntax{namespace: ns, kind: identityNameNamespace}, nil
	}
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return identityNameSyntax{}, err
	}
	ns := ""
	if parts.Prefixed {
		if ns, err = resolver.ResolveIdentityPrefix(parts.Prefix); err != nil {
			return identityNameSyntax{}, err
		}
	}
	return identityNameSyntax{namespace: ns, local: parts.Local, kind: identityNameQName}, nil
}

// parseIdentityStepNames appends the name tests of an element path to names,
// in step order, without interning them.
func parseIdentityStepNames[R identityNameResolver](path string, resolver R, names []identityNameSyntax) ([]identityNameSyntax, error) {
	if err := validateIdentityElementPath(path); err != nil {
		return nil, err
	}
	for rest, more := path, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "/")
		name, present, err := parseIdentityElementStep(part, resolver)
		if err != nil {
			return nil, err
		}
		if present {
			names = append(names, name)
		}
	}
	return names, nil
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

func parseIdentityElementStep[R identityNameResolver](part string, resolver R) (identityNameSyntax, bool, error) {
	part = lex.TrimXMLWhitespaceString(part)
	if part == "" {
		return identityNameSyntax{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity XPath step")
	}
	if strings.Contains(part, "::") {
		name, err := parseIdentityChildAxisStep(part)
		if err != nil {
			return identityNameSyntax{}, false, err
		}
		part = name
	}
	if part == "." {
		return identityNameSyntax{}, false, nil
	}
	name, err := parseIdentityNameSyntax(part, resolver)
	return name, true, err
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
