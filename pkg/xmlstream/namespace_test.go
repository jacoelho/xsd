package xmlstream

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestLookupNamespace(t *testing.T) {
	input := `<root xmlns="urn:root" xmlns:a="urn:a"><a:child xmlns=""><inner xmlns:b="urn:b"/></a:child></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if ns, ok := r.LookupNamespace(""); !ok || ns != "urn:root" {
		t.Fatalf("LookupNamespace default = %q, ok=%v, want urn:root, true", ns, ok)
	}
	if ns, ok := r.LookupNamespace("a"); !ok || ns != "urn:a" {
		t.Fatalf("LookupNamespace a = %q, ok=%v, want urn:a, true", ns, ok)
	}
	if ns, ok := r.LookupNamespace("xml"); !ok || ns != XMLNamespace {
		t.Fatalf("LookupNamespace xml = %q, ok=%v, want %q, true", ns, ok, XMLNamespace)
	}
	if _, ok := r.LookupNamespace("missing"); ok {
		t.Fatalf("LookupNamespace missing = true, want false")
	}
	if ns, ok := r.LookupNamespaceBytes([]byte("a")); !ok || ns != "urn:a" {
		t.Fatalf("LookupNamespaceBytes a = %q, ok=%v, want urn:a, true", ns, ok)
	}

	if _, err = r.Next(); err != nil { // child
		t.Fatalf("child start error = %v", err)
	}
	if ns, ok := r.LookupNamespace(""); !ok || ns != "" {
		t.Fatalf("LookupNamespace default child = %q, ok=%v, want empty, true", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("", 0); !ok || ns != "urn:root" {
		t.Fatalf("LookupNamespaceAt default root = %q, ok=%v, want urn:root, true", ns, ok)
	}
	if ns, ok := r.LookupNamespaceBytesAt([]byte("a"), 0); !ok || ns != "urn:a" {
		t.Fatalf("LookupNamespaceBytesAt a = %q, ok=%v, want urn:a, true", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("a", 99); !ok || ns != "urn:a" {
		t.Fatalf("LookupNamespaceAt depth overflow = %q, ok=%v, want urn:a, true", ns, ok)
	}
}

func TestNamespaceDeclsAt(t *testing.T) {
	input := `<root xmlns="urn:root" xmlns:a="urn:a"><a:child xmlns:b="urn:b"/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next() // root
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	rootDecls := declsToMap(r.NamespaceDeclsAt(ev.ScopeDepth))
	if rootDecls[""] != "urn:root" {
		t.Fatalf("root default namespace = %q, want urn:root", rootDecls[""])
	}
	if rootDecls["a"] != "urn:a" {
		t.Fatalf("root prefix a = %q, want urn:a", rootDecls["a"])
	}

	ev, err = r.Next() // child
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	childDecls := declsToMap(r.NamespaceDeclsAt(ev.ScopeDepth))
	if childDecls["b"] != "urn:b" {
		t.Fatalf("child prefix b = %q, want urn:b", childDecls["b"])
	}
}

func TestNamespaceDeclsAtDepthOverflow(t *testing.T) {
	input := `<root xmlns="urn:root" xmlns:a="urn:a"></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next() // root
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	decls := declsToMap(r.NamespaceDeclsAt(ev.ScopeDepth + 10))
	if decls[""] != "urn:root" {
		t.Fatalf("root default namespace = %q, want urn:root", decls[""])
	}
	if decls["a"] != "urn:a" {
		t.Fatalf("root prefix a = %q, want urn:a", decls["a"])
	}
}

func TestNamespaceDecls(t *testing.T) {
	input := `<root xmlns="urn:root" xmlns:a="urn:a"></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	decls := declsToMap(r.NamespaceDecls())
	if decls[""] != "urn:root" {
		t.Fatalf("root default namespace = %q, want urn:root", decls[""])
	}
	if decls["a"] != "urn:a" {
		t.Fatalf("root prefix a = %q, want urn:a", decls["a"])
	}
}

func TestNamespaceDeclsOrder(t *testing.T) {
	input := `<root xmlns="urn:default" xmlns:b="urn:b" xmlns:a="urn:a" xmlns:c="urn:c"></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	decls := r.NamespaceDeclsAt(ev.ScopeDepth)
	want := []NamespaceDecl{
		{Prefix: "", URI: "urn:default"},
		{Prefix: "b", URI: "urn:b"},
		{Prefix: "a", URI: "urn:a"},
		{Prefix: "c", URI: "urn:c"},
	}
	if len(decls) != len(want) {
		t.Fatalf("decls len = %d, want %d", len(decls), len(want))
	}
	for i, decl := range want {
		if decls[i] != decl {
			t.Fatalf("decl %d = %v, want %v", i, decls[i], decl)
		}
	}
}

func TestNamespaceDeclsNilReader(t *testing.T) {
	var r *Reader
	if decls := r.NamespaceDecls(); decls != nil {
		t.Fatalf("NamespaceDecls nil = %v, want nil", decls)
	}
	if decls := r.NamespaceDeclsAt(0); decls != nil {
		t.Fatalf("NamespaceDeclsAt nil = %v, want nil", decls)
	}
}

func TestNamespaceDeclsUndeclare(t *testing.T) {
	input := `<root xmlns="urn:root"><child xmlns=""/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next() // child
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	childDecls := declsToMap(r.NamespaceDeclsAt(ev.ScopeDepth))
	value, ok := childDecls[""]
	if !ok {
		t.Fatalf("child default namespace missing")
	}
	if value != "" {
		t.Fatalf("child default namespace = %q, want empty", value)
	}
}

func declsToMap(decls []NamespaceDecl) map[string]string {
	if len(decls) == 0 {
		return nil
	}
	out := make(map[string]string, len(decls))
	for _, decl := range decls {
		out[decl.Prefix] = decl.URI
	}
	return out
}

func TestNamespaceShadowing(t *testing.T) {
	input := `<root xmlns:a="urn:one"><a:child xmlns:a="urn:two"><a:inner/></a:child><a:after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next() // child
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.Name.Namespace != "urn:two" {
		t.Fatalf("child namespace = %q, want urn:two", ev.Name.Namespace)
	}
	ev, err = r.Next() // inner
	if err != nil {
		t.Fatalf("inner start error = %v", err)
	}
	if ev.Name.Namespace != "urn:two" {
		t.Fatalf("inner namespace = %q, want urn:two", ev.Name.Namespace)
	}
	if _, err = r.Next(); err != nil { // inner end
		t.Fatalf("inner end error = %v", err)
	}
	if _, err = r.Next(); err != nil { // child end
		t.Fatalf("child end error = %v", err)
	}
	ev, err = r.Next() // after
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Name.Namespace != "urn:one" {
		t.Fatalf("after namespace = %q, want urn:one", ev.Name.Namespace)
	}
}

func TestNamespaceUndeclareRedeclare(t *testing.T) {
	input := `<root xmlns="urn:root"><child xmlns=""><inner xmlns="urn:inner"/></child></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next() // root
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Name.Namespace != "urn:root" {
		t.Fatalf("root namespace = %q, want urn:root", ev.Name.Namespace)
	}
	ev, err = r.Next() // child
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.Name.Namespace != "" {
		t.Fatalf("child namespace = %q, want empty", ev.Name.Namespace)
	}
	ev, err = r.Next() // inner
	if err != nil {
		t.Fatalf("inner start error = %v", err)
	}
	if ev.Name.Namespace != "urn:inner" {
		t.Fatalf("inner namespace = %q, want urn:inner", ev.Name.Namespace)
	}
}

func TestCollectNamespaceScopeAllocPrefixes(t *testing.T) {
	input := `<root xmlns:p="urn:p"><p:child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Namespace != "urn:p" {
		t.Fatalf("child namespace = %q, want urn:p", ev.Name.Namespace)
	}
}

func TestDefaultNamespaceDeclPrefixMatch(t *testing.T) {
	if isDefaultNamespaceDecl([]byte("xmln")) {
		t.Fatalf("isDefaultNamespaceDecl(xmln) = true, want false")
	}
	if isDefaultNamespaceDecl([]byte("xmlns")) != true {
		t.Fatalf("isDefaultNamespaceDecl(xmlns) = false, want true")
	}
}

func TestPrefixedNamespaceDeclXML(t *testing.T) {
	if local, ok := prefixedNamespaceDecl([]byte("xmlns:xml")); ok || local != nil {
		t.Fatalf("prefixedNamespaceDecl(xmlns:xml) = %q, ok=%v, want nil, false", local, ok)
	}
}

func TestNamespaceDeepShadowing(t *testing.T) {
	const levels = 120
	var b strings.Builder
	fmt.Fprintf(&b, `<p:e0 xmlns:p="urn:0">`)
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&b, `<p:e%d xmlns:p="urn:%d">`, i, i)
	}
	for i := levels - 1; i >= 1; i-- {
		fmt.Fprintf(&b, `</p:e%d>`, i)
	}
	b.WriteString(`<p:after/>`)
	b.WriteString(`</p:e0>`)
	r, err := NewReader(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	expect := make([]string, 0, levels+1)
	for i := range levels {
		expect = append(expect, fmt.Sprintf("urn:%d", i))
	}
	expect = append(expect, "urn:0")
	var seen int
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		if ev.Kind != EventStartElement {
			continue
		}
		if seen >= len(expect) {
			t.Fatalf("extra start element %s", ev.Name.String())
		}
		if ev.Name.Namespace != expect[seen] {
			t.Fatalf("namespace %d = %q, want %q", seen, ev.Name.Namespace, expect[seen])
		}
		seen++
	}
	if seen != len(expect) {
		t.Fatalf("start elements = %d, want %d", seen, len(expect))
	}
}

func TestUnboundPrefixElement(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root><a:child/></root>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("unbound prefix error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("unbound prefix error type = %T, want *xmltext.SyntaxError", err)
	}
	if !errors.Is(err, errUnboundPrefix) {
		t.Fatalf("unbound prefix error = %v, want %v", err, errUnboundPrefix)
	}
}

func TestUnboundPrefixAttr(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root><child a:attr=\"v\"/></root>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("unbound prefix error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("unbound prefix attr error type = %T, want *xmltext.SyntaxError", err)
	}
	if !errors.Is(err, errUnboundPrefix) {
		t.Fatalf("unbound prefix attr error = %v, want %v", err, errUnboundPrefix)
	}
}

func TestNamespaceValueEntities(t *testing.T) {
	input := `<root xmlns="urn:a&amp;b"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Name.Namespace != "urn:a&b" {
		t.Fatalf("root namespace = %q, want urn:a&b", ev.Name.Namespace)
	}
	if ns, ok := r.LookupNamespace(""); !ok || ns != "urn:a&b" {
		t.Fatalf("LookupNamespace default = %q, ok=%v, want urn:a&b, true", ns, ok)
	}
}

func TestNamespaceValueWhitespaceEntities(t *testing.T) {
	input := `<root xmlns="&#x20;&#x9;"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Name.Namespace != " \t" {
		t.Fatalf("root namespace = %q, want \" \\t\"", ev.Name.Namespace)
	}
	if ns, ok := r.LookupNamespace(""); !ok || ns != " \t" {
		t.Fatalf("LookupNamespace default = %q, ok=%v, want \" \\t\", true", ns, ok)
	}
}

func TestNamespaceValueNumericEntities(t *testing.T) {
	input := `<root xmlns="urn:a&#x26;b"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Name.Namespace != "urn:a&b" {
		t.Fatalf("root namespace = %q, want urn:a&b", ev.Name.Namespace)
	}
	if ns, ok := r.LookupNamespace(""); !ok || ns != "urn:a&b" {
		t.Fatalf("LookupNamespace default = %q, ok=%v, want urn:a&b, true", ns, ok)
	}
}

func TestLargeNamespaceURI(t *testing.T) {
	ns := "urn:test:" + strings.Repeat("x", 1024)
	input := fmt.Sprintf(`<root xmlns=%q><child/></root>`, ns)
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Name.Namespace != ns {
		t.Fatalf("namespace len = %d, want %d", len(ev.Name.Namespace), len(ns))
	}
}

func TestNSStackPopEmpty(t *testing.T) {
	var stack nsStack
	stack.pop()
	if stack.depth() != 0 {
		t.Fatalf("depth = %d, want 0", stack.depth())
	}
}

func TestEmptyPrefixDeclaration(t *testing.T) {
	input := `<root xmlns:="urn:test"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("empty prefix decl error = nil, want error")
	}
}

func TestSplitQNameEdgeCases(t *testing.T) {
	tests := []struct {
		input     string
		prefix    string
		local     string
		hasPrefix bool
	}{
		{input: "local", prefix: "", local: "local", hasPrefix: false},
		{input: "p:local", prefix: "p", local: "local", hasPrefix: true},
		{input: ":local", prefix: "", local: "local", hasPrefix: true},
		{input: "prefix:", prefix: "prefix", local: "", hasPrefix: true},
		{input: "a:b:c", prefix: "a", local: "b:c", hasPrefix: true},
	}
	for _, tt := range tests {
		prefix, local, hasPrefix := splitQName([]byte(tt.input))
		if hasPrefix != tt.hasPrefix {
			t.Fatalf("splitQName(%q) hasPrefix = %v, want %v", tt.input, hasPrefix, tt.hasPrefix)
		}
		if got := string(prefix); got != tt.prefix {
			t.Fatalf("splitQName(%q) prefix = %q, want %q", tt.input, got, tt.prefix)
		}
		if got := string(local); got != tt.local {
			t.Fatalf("splitQName(%q) local = %q, want %q", tt.input, got, tt.local)
		}
	}
}

func TestSplitQNameEmpty(t *testing.T) {
	prefix, local, hasPrefix := splitQName(nil)
	if hasPrefix {
		t.Fatalf("splitQName(nil) hasPrefix = true, want false")
	}
	if prefix != nil {
		t.Fatalf("splitQName(nil) prefix = %v, want nil", prefix)
	}
	if local != nil {
		t.Fatalf("splitQName(nil) local = %v, want nil", local)
	}
}

func TestNSStackLookupEmptyStack(t *testing.T) {
	var stack nsStack
	if ns, ok := stack.lookup("a", 0); ok || ns != "" {
		t.Fatalf("lookup empty stack = %q, ok=%v, want empty, false", ns, ok)
	}
}

func TestNSStackLookupNegativeDepth(t *testing.T) {
	var stack nsStack
	stack.decls = append(stack.decls, NamespaceDecl{Prefix: "a", URI: "urn:a"})
	stack.push(nsScope{declStart: 0, declLen: 1})
	if ns, ok := stack.lookup("a", -1); ok || ns != "" {
		t.Fatalf("lookup negative depth = %q, ok=%v, want empty, false", ns, ok)
	}
}

func TestLookupNamespaceNilReader(t *testing.T) {
	var r *Reader
	if ns, ok := r.LookupNamespace(""); ok || ns != "" {
		t.Fatalf("LookupNamespace nil = %q, ok=%v, want empty, false", ns, ok)
	}
	if ns, ok := r.LookupNamespaceBytes([]byte("p")); ok || ns != "" {
		t.Fatalf("LookupNamespaceBytes nil = %q, ok=%v, want empty, false", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("p", 0); ok || ns != "" {
		t.Fatalf("LookupNamespaceAt nil = %q, ok=%v, want empty, false", ns, ok)
	}
	if ns, ok := r.LookupNamespaceBytesAt([]byte("p"), 0); ok || ns != "" {
		t.Fatalf("LookupNamespaceBytesAt nil = %q, ok=%v, want empty, false", ns, ok)
	}
}

func TestUnboundPrefixErrorNilDecoder(t *testing.T) {
	err := unboundPrefixError(nil, 3, 4)
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("unboundPrefixError type = %T, want *xmltext.SyntaxError", err)
	}
	if !errors.Is(err, errUnboundPrefix) {
		t.Fatalf("unboundPrefixError = %v, want %v", err, errUnboundPrefix)
	}
	if syntax.Line != 3 || syntax.Column != 4 {
		t.Fatalf("unboundPrefixError line/column = %d:%d, want 3:4", syntax.Line, syntax.Column)
	}
}

func TestXMLNSXMLBindingIgnored(t *testing.T) {
	input := `<root xmlns:xml="urn:wrong"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ns, ok := r.LookupNamespace("xml"); !ok || ns != XMLNamespace {
		t.Fatalf("LookupNamespace xml = %q, ok=%v, want %q, true", ns, ok, XMLNamespace)
	}
}

func TestXMLNSXMLNSBindingIgnored(t *testing.T) {
	input := `<root xmlns:xmlns="urn:wrong"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, ok := r.LookupNamespace("xmlns"); ok {
		t.Fatalf("LookupNamespace xmlns = true, want false")
	}
}

func TestNSStackLookupDepthOverflow(t *testing.T) {
	var stack nsStack
	stack.decls = append(stack.decls, NamespaceDecl{Prefix: "a", URI: "urn:a"})
	stack.push(nsScope{declStart: 0, declLen: 1})
	if ns, ok := stack.lookup("a", 10); !ok || ns != "urn:a" {
		t.Fatalf("lookup overflow = %q, ok=%v, want urn:a, true", ns, ok)
	}
}

func TestCollectNamespaceScopePrefixedError(t *testing.T) {
	input := `<root xmlns:a="&bad;"><a:child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("prefixed namespace error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("prefixed namespace error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestCollectNamespaceScopeDefaultError(t *testing.T) {
	input := `<root xmlns="&bad;"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("default namespace error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("default namespace error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestNamespaceTruncatedNumericRef(t *testing.T) {
	input := `<root xmlns="&#"></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("truncated namespace ref error = nil, want error")
	}
}

func TestPrefixedNamespaceTruncatedEntity(t *testing.T) {
	input := `<root xmlns:p="&#"><p:child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("truncated prefixed namespace error = nil, want error")
	}
}

func TestCollectNamespaceScopeMultiplePrefixes(t *testing.T) {
	input := `<root xmlns:a="urn:a" xmlns:b="urn:b" xmlns:c="urn:c"><a:child b:attr="v" c:attr="w"/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ns, ok := r.LookupNamespaceAt("a", ev.ScopeDepth); !ok || ns != "urn:a" {
		t.Fatalf("LookupNamespaceAt a = %q, ok=%v, want urn:a, true", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("b", ev.ScopeDepth); !ok || ns != "urn:b" {
		t.Fatalf("LookupNamespaceAt b = %q, ok=%v, want urn:b, true", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("c", ev.ScopeDepth); !ok || ns != "urn:c" {
		t.Fatalf("LookupNamespaceAt c = %q, ok=%v, want urn:c, true", ns, ok)
	}
	child, err := r.Next()
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if child.Name.Namespace != "urn:a" || child.Name.Local != "child" {
		t.Fatalf("child name = %s, want {urn:a}child", child.Name.String())
	}
	var seenB bool
	var seenC bool
	for _, attr := range child.Attrs {
		switch {
		case attr.Name.Namespace == "urn:b" && attr.Name.Local == "attr":
			seenB = true
			if string(attr.Value) != "v" {
				t.Fatalf("b:attr value = %q, want v", string(attr.Value))
			}
		case attr.Name.Namespace == "urn:c" && attr.Name.Local == "attr":
			seenC = true
			if string(attr.Value) != "w" {
				t.Fatalf("c:attr value = %q, want w", string(attr.Value))
			}
		}
	}
	if !seenB || !seenC {
		t.Fatalf("attrs seen b=%v c=%v", seenB, seenC)
	}
}

func TestResolveAttrNameBareXMLNS(t *testing.T) {
	dec := xmltext.NewDecoder(strings.NewReader("<root/>"))
	ns := nsStack{}
	namespace, local, err := resolveAttrName(dec, &ns, []byte("xmlns"), -1, 0, 1, 1)
	if err != nil {
		t.Fatalf("resolveAttrName error = %v", err)
	}
	if namespace != XMLNSNamespace {
		t.Fatalf("namespace = %q, want %q", namespace, XMLNSNamespace)
	}
	if string(local) != "xmlns" {
		t.Fatalf("local = %q, want xmlns", string(local))
	}
}
