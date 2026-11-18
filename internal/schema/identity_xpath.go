package schema

import (
	"slices"
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
// branches and their bound name tests are kept for the compiler to intern
// (identityXPathSyntax), so name IDs follow compilation order, as every other
// component name's do, and a MaxSchemaNames breach on a path name is a compile
// diagnostic that no syntax or binding error of any path waits behind.
func admitIdentityXPath(n *schemaSyntaxNode, value string, values *admittedValues) error {
	resolver := tagIdentityPrefixResolver{node: n}
	var err error
	switch n.Name.Local {
	case vocab.XSDElemSelector:
		values.xpath, err = parseIdentityPaths(value, resolver)
	case vocab.XSDElemField:
		values.xpath, err = parseIdentityFieldPaths(value, resolver)
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

// identityNameSyntax is an item of an admitted identity XPath
// (identityXPathSyntax): a name test whose prefix is bound and whose names
// are not interned yet, or the whole of a branch without name tests. The
// first item of a branch is marked start and carries the branch's
// descendant prefix and whether it ends in an attribute.
type identityNameSyntax struct {
	namespace  string
	local      string
	kind       identityNameKind
	start      bool
	descendant bool
	attribute  bool
}

// identityNameKind is the form of an identity XPath item. Its zero value is
// no form, so an item that was never parsed cannot pass for one.
type identityNameKind uint8

const (
	identityNameInvalid identityNameKind = iota
	// identityBranch is a branch without name tests, such as ".".
	identityBranch
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
	case identityNameInvalid, identityBranch:
		return identityNameTest{}, xsderrors.InternalInvariant("identity XPath item is not a name test")
	default:
		return identityNameTest{}, xsderrors.InternalInvariant("unknown identity XPath name test kind")
	}
}

// identityStepFor returns the element step of the interned name test test.
func identityStepFor(test identityNameTest) IdentityStep {
	return IdentityStep{Name: test.name, Namespace: test.namespace, Wildcard: test.wildcard, NamespaceSet: test.namespaceSet}
}

// identityXPathSyntax is a selector or field path as its start tag admits it,
// in one slice: the name tests of each branch in turn, none interned; in a
// field branch that ends in an attribute, the attribute's name test comes
// first. It is part of the immutable parsed document; compilers build
// interned paths from it without changing it.
type identityXPathSyntax struct {
	items []identityNameSyntax
}

// branches returns the number of branches of s.
func (s identityXPathSyntax) branches() int {
	count := 0
	for _, item := range s.items {
		if item.start {
			count++
		}
	}
	return count
}

// identityBranchItems is one branch of an identityXPathSyntax.
type identityBranchItems struct {
	// names are the branch's name tests, the attribute's first.
	names      []identityNameSyntax
	end        int
	descendant bool
	attribute  bool
}

// branchAt returns the branch that starts at s.items[at]; its end is the
// index of the item after it.
func (s identityXPathSyntax) branchAt(at int) (identityBranchItems, error) {
	if at >= len(s.items) || !s.items[at].start {
		return identityBranchItems{}, xsderrors.InternalInvariant("identity XPath item does not start a branch")
	}
	end := at + 1
	for end < len(s.items) && !s.items[end].start {
		end++
	}
	head := s.items[at]
	branch := identityBranchItems{names: s.items[at:end], end: end, descendant: head.descendant, attribute: head.attribute}
	if head.kind == identityBranch {
		if end != at+1 || head.attribute {
			return identityBranchItems{}, xsderrors.InternalInvariant("identity XPath branch without name tests has items")
		}
		branch.names = nil
	}
	return branch, nil
}

// selectorPaths interns the name tests of s, a selector, into table and
// returns its paths.
func (s identityXPathSyntax) selectorPaths(table *nameTable) ([]identityPath, error) {
	paths := make([]identityPath, s.branches())
	next := 0
	for i := range paths {
		branch, err := s.branchAt(next)
		if err != nil {
			return nil, err
		}
		if branch.attribute {
			return nil, xsderrors.InternalInvariant("identity selector branch ends in an attribute")
		}
		steps, err := internIdentitySteps(table, branch.names)
		if err != nil {
			return nil, err
		}
		paths[i] = identityPath{Descendant: branch.descendant, Steps: steps}
		next = branch.end
	}
	return paths, nil
}

// fieldPaths interns the name tests of s, a field, into table and returns its
// paths.
func (s identityXPathSyntax) fieldPaths(table *nameTable) ([]identityFieldPath, error) {
	paths := make([]identityFieldPath, s.branches())
	next := 0
	for i := range paths {
		branch, err := s.branchAt(next)
		if err != nil {
			return nil, err
		}
		names := branch.names
		path := identityFieldPath{Descendant: branch.descendant, Attr: branch.attribute, Attribute: noQName()}
		if branch.attribute {
			test, nameErr := internIdentityName(table, names[0])
			if nameErr != nil {
				return nil, nameErr
			}
			path.Attribute, path.AttrNamespace, path.AttrWildcard, path.AttrNamespaceSet = test.name, test.namespace, test.wildcard, test.namespaceSet
			names = names[1:]
		}
		if path.Steps, err = internIdentitySteps(table, names); err != nil {
			return nil, err
		}
		paths[i] = path
		next = branch.end
	}
	return paths, nil
}

// internIdentitySteps interns the element step name tests names into table.
func internIdentitySteps(table *nameTable, names []identityNameSyntax) ([]IdentityStep, error) {
	if len(names) == 0 {
		return nil, nil
	}
	steps := make([]IdentityStep, len(names))
	for i, name := range names {
		test, err := internIdentityName(table, name)
		if err != nil {
			return nil, err
		}
		steps[i] = identityStepFor(test)
	}
	return steps, nil
}

// startIdentityBranch marks the items of a branch appended from start on, or
// appends a branch without name tests, with the branch's form.
func startIdentityBranch(items []identityNameSyntax, start int, descendant, attribute bool) []identityNameSyntax {
	if len(items) == start {
		items = append(items, identityNameSyntax{kind: identityBranch})
	}
	items[start].start, items[start].descendant, items[start].attribute = true, descendant, attribute
	return items
}

// parseIdentityPaths parses the branches of a selector XPath and their name
// tests. Like the field parse, it reserves at most maxIdentityNameHint items
// from the separator count before accepting any branch and grows only with
// accepted branches and name tests. The items are collected on the stack when
// they can be few and copied out at exactly their length, because the source
// record keeps them, and otherwise in one reservation.
func parseIdentityPaths[R identityPrefixResolver](xpath string, resolver R) (identityXPathSyntax, error) {
	if hint := identityItemHint(xpath); hint > identityStackItems {
		items, err := parseIdentityPathsInto(xpath, resolver, make([]identityNameSyntax, 0, hint))
		return identityXPathSyntax{items: items}, err
	}
	var buffer [identityStackItems]identityNameSyntax
	items, err := parseIdentityPathsInto(xpath, resolver, buffer[:0])
	if err != nil {
		return identityXPathSyntax{}, err
	}
	return identityXPathSyntax{items: slices.Clone(items)}, nil
}

// identityStackItems is the most items a path collects on the stack.
const identityStackItems = 8

func parseIdentityPathsInto[R identityPrefixResolver](xpath string, resolver R, items []identityNameSyntax) ([]identityNameSyntax, error) {
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
		start := len(items)
		var err error
		if items, err = parseIdentityStepNames(part, resolver, items); err != nil {
			return nil, err
		}
		items = startIdentityBranch(items, start, desc, false)
	}
	return items, nil
}

// maxIdentityItemHint bounds the items a path reserves before its branches
// are accepted, so a malformed path with many separators is rejected without
// allocating per separator.
const maxIdentityItemHint = 256

// identityItemHint is the number of items xpath holds at most, capped at
// maxIdentityItemHint: a branch holds at most one item more than its step
// separators, since an attribute ends the branch's last step.
func identityItemHint(xpath string) int {
	return min(strings.Count(xpath, "|")+strings.Count(xpath, "/")+1, maxIdentityItemHint)
}

// parseIdentityFieldPaths parses the branches of a field XPath and their name
// tests: in each branch its attribute's, then its steps'.
func parseIdentityFieldPaths[R identityPrefixResolver](xpath string, resolver R) (identityXPathSyntax, error) {
	if hint := identityItemHint(xpath); hint > identityStackItems {
		items, err := parseIdentityFieldPathsInto(xpath, resolver, make([]identityNameSyntax, 0, hint))
		return identityXPathSyntax{items: items}, err
	}
	var buffer [identityStackItems]identityNameSyntax
	items, err := parseIdentityFieldPathsInto(xpath, resolver, buffer[:0])
	if err != nil {
		return identityXPathSyntax{}, err
	}
	return identityXPathSyntax{items: slices.Clone(items)}, nil
}

func parseIdentityFieldPathsInto[R identityPrefixResolver](xpath string, resolver R, items []identityNameSyntax) ([]identityNameSyntax, error) {
	for rest, more := xpath, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, "|")
		part = lex.TrimXMLWhitespaceString(part)
		if part == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
		}
		var err error
		if items, err = parseIdentityFieldPathBranch(part, resolver, items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func parseIdentityFieldPathBranch[R identityPrefixResolver](part string, resolver R, items []identityNameSyntax) ([]identityNameSyntax, error) {
	desc := false
	if rest, ok := parseIdentityDescendantPrefix(part); ok {
		desc = true
		part = rest
	}
	if part == "" {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity field XPath branch is empty")
	}
	attribute, err := parseIdentityFieldAttribute(part, resolver)
	if err != nil {
		return nil, err
	}
	start := len(items)
	if attribute.present {
		items = append(items, attribute.name)
	}
	if attribute.elementPath != "" {
		if items, err = parseIdentityStepNames(attribute.elementPath, resolver, items); err != nil {
			return nil, err
		}
	}
	return startIdentityBranch(items, start, desc, attribute.present), nil
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
