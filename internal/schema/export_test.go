package schema

import (
	"bytes"
	"slices"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

type Compiler = compiler

type SchemaBuild = schemaBuild

var PublishSchema = publishSchema

// ValidateBuildForTest checks construction invariants without consuming the
// value builder, so fault-injection tests can validate then mutate a fixture.
func ValidateBuildForTest(build *schemaBuild) error {
	analysis, err := NewContentModelAnalysis(build, unlimitedContentModelWork)
	if err != nil {
		return err
	}
	return validateSchema(&schemaValidation{build: build, contentModelWork: unlimitedContentModelWork, contentAnalysis: analysis})
}

// SchemaNode exposes parsed semantic nodes to package-boundary white-box tests.
type SchemaNode = schemaNode

// NewCompilerForTest creates a compiler for package-boundary regression tests.
func NewCompilerForTest(limits Limits) (*Compiler, error) {
	return newCompiler(limits)
}

// LoadForTest loads schema sources into the compiler for white-box tests.
func (c *compiler) LoadForTest(sources []source.Source) error {
	return c.loadOwned(slices.Clone(sources))
}

// IndexForTest indexes loaded schema documents for white-box tests.
func (c *compiler) IndexForTest() error {
	return c.index()
}

// CompileGlobalsForTest compiles indexed global declarations for white-box tests.
func (c *compiler) CompileGlobalsForTest() error {
	return c.compileGlobals()
}

// RuntimeForTest returns the compiler-owned mutable runtime for white-box tests.
func (c *compiler) RuntimeForTest() *schemaBuild {
	return &c.rt
}

// NameInternerIsZeroForTest reports whether the compiler name interner was cleared.
func (c *compiler) NameInternerIsZeroForTest() bool {
	return c.rt.Names.NameCount() == 0
}

// DocumentNamesForTest returns loaded schema document names in compiler order.
func (c *compiler) DocumentNamesForTest() []string {
	names := make([]string, 0, len(c.plan.documents))
	for _, document := range c.plan.documents {
		names = append(names, document.doc.name)
	}
	return names
}

// ParseSchemaRootForTest parses a schema document and returns its root node.
func ParseSchemaRootForTest(data []byte, limits Limits) (*SchemaNode, error) {
	doc, err := parseTypedSchemaSourceDocument("test.xsd", "test.xsd", bytes.NewReader(data), limits, new(xmlstream.Reader))
	if err != nil {
		return nil, err
	}
	return doc.root, nil
}

// ChildForTest returns a semantic child by document order for package-boundary
// parser tests without exposing the source arena representation.
func (n *schemaNode) ChildForTest(index int) *schemaNode {
	if n == nil || index < 0 || index >= len(n.children) || n.doc == nil {
		return nil
	}
	return n.doc.node(n.children[index])
}

// ChildrenForTest returns semantic children in document order for parser
// tests. The returned slice is a read-only snapshot.
func (n *schemaNode) ChildrenForTest() []*schemaNode {
	if n == nil || n.doc == nil {
		return nil
	}
	children := make([]*schemaNode, 0, len(n.children))
	for _, id := range n.children {
		if child := n.doc.node(id); child != nil {
			children = append(children, child)
		}
	}
	return children
}

// NamespaceLookupForTest exposes one retained namespace lookup for parser
// isolation tests. Nodes without deferred QName interpretation intentionally
// return no bindings.
func (n *schemaNode) NamespaceLookupForTest(prefix string) (string, bool) {
	if n == nil {
		return "", false
	}
	return n.namespace.Lookup(prefix)
}

// FreezeCompilerRuntimeForTest freezes a compiler runtime for white-box tests.
func FreezeCompilerRuntimeForTest(c *Compiler) (*Schema, error) {
	return c.publishSchema()
}
