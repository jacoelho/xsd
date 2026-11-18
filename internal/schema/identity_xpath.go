package schema

import (
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// identityPrefixResolver binds the prefixes of identity XPath name tests.
type identityPrefixResolver interface {
	ResolveIdentityPrefix(prefix string) (string, error)
}

// admitIdentityXPath parses value, the xpath of the selector or field n, into
// values: the path grammar and the prefix binding of each name test, branch by
// branch from the left, so the first violating branch is the one reported. It
// is the only owner of those diagnostics and the only parse of the value.
// Prefixes resolve against the namespace scope of the start tag; an
// unprefixed name test has no namespace (XPath 1.0 §2.3), whatever the
// target namespace of the including context. No name is interned here: the
// name tests are kept, bound, for compileIdentityConstraint to intern, so name
// IDs follow compilation order, as every other component name's do, and a
// MaxSchemaNames breach on a path name is a compile diagnostic that no syntax
// or binding error of any path waits behind.
func admitIdentityXPath(n *schemaSyntaxNode, value string, values *admittedValues) error {
	resolver := tagIdentityPrefixResolver{node: n}
	var err error
	switch n.Name.Local {
	case vocab.XSDElemSelector:
		values.selector, values.xpathNames, err = parseIdentityPaths(value, resolver)
	case vocab.XSDElemField:
		values.field, values.xpathNames, err = parseIdentityFieldPaths(value, resolver)
	default:
		err = xsderrors.InternalInvariant("identity XPath admission of xs:" + diagnostictext.Quote(n.Name.Local))
	}
	return withSchemaCompileLocation(n, err)
}

// tagIdentityPrefixResolver resolves the prefixes of name tests against the
// namespace scope of a start tag.
type tagIdentityPrefixResolver struct {
	node *schemaSyntaxNode
}

func (r tagIdentityPrefixResolver) ResolveIdentityPrefix(prefix string) (string, error) {
	ns, ok := r.node.NS.Lookup(prefix)
	if !ok {
		return "", schemaCompileAt(r.node, xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(prefix))
	}
	return ns, nil
}

// identityNameSyntax is a name test whose prefix is bound and whose names are
// not interned yet.
type identityNameSyntax struct {
	namespace string
	local     string
	kind      identityNameKind
}

// identityNameKind is the form of a name test. Its zero value is no form, so
// a name test that was never parsed cannot pass for one.
type identityNameKind uint8

const (
	identityNameInvalid identityNameKind = iota
	// identityNameAny is the name test *.
	identityNameAny
	// identityNameNamespace is the name test prefix:*.
	identityNameNamespace
	// identityNameQName is a QName name test.
	identityNameQName
)

// internIdentityName interns the names of syntax into names.
func internIdentityName(names *nameTable, syntax identityNameSyntax) (identityNameTest, error) {
	interner := newNameInterner(names)
	switch syntax.kind {
	case identityNameNamespace:
		ns, err := interner.InternNamespace(syntax.namespace)
		if err != nil {
			return identityNameTest{}, err
		}
		return identityNameTest{name: noQName(), wildcard: true, namespaceSet: true, namespace: ns}, nil
	case identityNameQName:
		q, err := interner.InternQName(syntax.namespace, syntax.local)
		if err != nil {
			return identityNameTest{}, err
		}
		return identityNameTest{name: q}, nil
	case identityNameAny:
		return identityNameTest{name: noQName(), wildcard: true}, nil
	case identityNameInvalid:
		return identityNameTest{}, xsderrors.InternalInvariant("identity XPath name test was not parsed")
	default:
		return identityNameTest{}, xsderrors.InternalInvariant("unknown identity XPath name test kind")
	}
}

// identityStepFor returns the element step of the interned name test test.
func identityStepFor(test identityNameTest) IdentityStep {
	return IdentityStep{Name: test.name, Namespace: test.namespace, Wildcard: test.wildcard, NamespaceSet: test.namespaceSet}
}

// internIdentitySelectorNames interns names, the name tests
// parseIdentityPaths returned with paths, into the steps of paths in place.
func internIdentitySelectorNames(table *nameTable, paths []identityPath, names []identityNameSyntax) error {
	next := 0
	for _, path := range paths {
		for i := range path.Steps {
			if next == len(names) {
				return xsderrors.InternalInvariant("identity selector has more steps than admitted name tests")
			}
			test, err := internIdentityName(table, names[next])
			if err != nil {
				return err
			}
			path.Steps[i] = identityStepFor(test)
			next++
		}
	}
	if next != len(names) {
		return xsderrors.InternalInvariant("identity selector has fewer steps than admitted name tests")
	}
	return nil
}

// internIdentityFieldNames interns names, the name tests
// parseIdentityFieldPaths returned with paths, into the attributes and steps
// of paths in place: in each path its attribute, then its steps.
func internIdentityFieldNames(table *nameTable, paths []identityFieldPath, names []identityNameSyntax) error {
	next := 0
	for p := range paths {
		var err error
		if next, err = internIdentityFieldPathNames(table, &paths[p], names, next); err != nil {
			return err
		}
	}
	if next != len(names) {
		return xsderrors.InternalInvariant("identity field has fewer name tests than admitted")
	}
	return nil
}

// internIdentityFieldPathNames interns the name tests of path, which start at
// names[next], and returns the index after them.
func internIdentityFieldPathNames(table *nameTable, path *identityFieldPath, names []identityNameSyntax, next int) (int, error) {
	count := len(path.Steps)
	if path.Attr {
		count++
	}
	if count > len(names)-next {
		return next, xsderrors.InternalInvariant("identity field has more name tests than admitted")
	}
	if path.Attr {
		test, err := internIdentityName(table, names[next])
		if err != nil {
			return next, err
		}
		path.Attribute, path.AttrNamespace, path.AttrWildcard, path.AttrNamespaceSet = test.name, test.namespace, test.wildcard, test.namespaceSet
		next++
	}
	for i := range path.Steps {
		test, err := internIdentityName(table, names[next])
		if err != nil {
			return next, err
		}
		path.Steps[i] = identityStepFor(test)
		next++
	}
	return next, nil
}

// identityPathNames is the number of name tests a path holds before its
// pending names spill from the stack.
const identityPathNames = 8

// parseIdentityPaths parses selector XPath branches for an identity constraint
// into its paths, whose steps hold no names yet, and their name tests in step
// order. Like the field parse, it reserves at most maxIdentityPathHint
// branches from the separator count before accepting any, grows only with
// accepted branches and name tests, and returns slices of exactly the accepted
// length, because the source record keeps them.
func parseIdentityPaths[R identityPrefixResolver](xpath string, resolver R) ([]identityPath, []identityNameSyntax, error) {
	out := make([]identityPath, 0, identityPathHint(xpath, "|"))
	var buffer [identityPathNames]identityNameSyntax
	names := buffer[:0]
	for rest, more := xpath, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "|")
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity selector XPath branch is empty")
		}
		desc := false
		if rest, ok := parseIdentityDescendantPrefix(part); ok {
			desc = true
			part = rest
		}
		start := len(names)
		var err error
		if names, err = parseIdentityStepNames(part, resolver, names); err != nil {
			return nil, nil, err
		}
		out = append(out, identityPath{Descendant: desc, Steps: unnamedIdentitySteps(len(names) - start)})
	}
	return tightSlice(out), ownedIdentityNames(names), nil
}

// ownedIdentityNames returns a copy of names, which may lie in a parse's stack
// buffer, of exactly their length.
func ownedIdentityNames(names []identityNameSyntax) []identityNameSyntax {
	if len(names) == 0 {
		return nil
	}
	return append(make([]identityNameSyntax, 0, len(names)), names...)
}

// unnamedIdentitySteps returns count steps whose names are not interned yet.
func unnamedIdentitySteps(count int) []IdentityStep {
	if count == 0 {
		return nil
	}
	return make([]IdentityStep, count)
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

// parseIdentityFieldPaths parses field XPath branches for an identity
// constraint into its paths, whose attributes and steps hold no names yet, and
// their name tests: in each path its attribute's, then its steps'.
func parseIdentityFieldPaths[R identityPrefixResolver](xpath string, resolver R) ([]identityFieldPath, []identityNameSyntax, error) {
	out := make([]identityFieldPath, 0, identityPathHint(xpath, "|"))
	var buffer [identityPathNames]identityNameSyntax
	names := buffer[:0]
	for rest, more := xpath, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "|")
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
		}
		var path identityFieldPath
		var err error
		if path, names, err = parseIdentityFieldPathBranch(part, resolver, names); err != nil {
			return nil, nil, err
		}
		out = append(out, path)
	}
	return tightSlice(out), ownedIdentityNames(names), nil
}

func parseIdentityFieldPathBranch[R identityPrefixResolver](part string, resolver R, names []identityNameSyntax) (identityFieldPath, []identityNameSyntax, error) {
	desc := false
	if rest, ok := parseIdentityDescendantPrefix(part); ok {
		desc = true
		part = rest
	}
	if part == "" {
		return identityFieldPath{}, nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
	}
	attribute, err := parseIdentityFieldAttribute(part, resolver)
	if err != nil {
		return identityFieldPath{}, nil, err
	}
	if attribute.present {
		names = append(names, attribute.name)
	}
	start := len(names)
	if attribute.elementPath != "" {
		if names, err = parseIdentityStepNames(attribute.elementPath, resolver, names); err != nil {
			return identityFieldPath{}, nil, err
		}
	}
	return identityFieldPath{
		Descendant: desc,
		Attr:       attribute.present,
		Steps:      unnamedIdentitySteps(len(names) - start),
		Attribute:  noQName(),
	}, names, nil
}

type identityFieldAttribute struct {
	elementPath string
	name        identityNameSyntax
	present     bool
}

func parseIdentityFieldAttribute[R identityPrefixResolver](part string, resolver R) (identityFieldAttribute, error) {
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

func parseIdentityNameSyntax[R identityPrefixResolver](lexical string, resolver R) (identityNameSyntax, error) {
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
func parseIdentityStepNames[R identityPrefixResolver](path string, resolver R, names []identityNameSyntax) ([]identityNameSyntax, error) {
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

func parseIdentityElementStep[R identityPrefixResolver](part string, resolver R) (identityNameSyntax, bool, error) {
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
