package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
)

type Compiler = compiler

// RawNode exposes parsed schema nodes to package-boundary white-box tests.
type RawNode = rawNode

// NewCompilerForTest creates a compiler for package-boundary regression tests.
func NewCompilerForTest(limits Limits) (*Compiler, error) {
	return newCompiler(limits)
}

// LoadForTest loads schema sources into the compiler for white-box tests.
func (c *compiler) LoadForTest(sources []source.Source) error {
	return c.load(sources)
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
func (c *compiler) RuntimeForTest() *runtime.SchemaBuild {
	return &c.rt.build
}

// NameInternerIsZeroForTest reports whether the compiler name interner was cleared.
func (c *compiler) NameInternerIsZeroForTest() bool {
	return c.rt.build.Names.NameCount() == 0
}

// DocumentNamesForTest returns loaded schema document names in compiler order.
func (c *compiler) DocumentNamesForTest() []string {
	names := make([]string, 0, len(c.schemas.documents))
	for _, document := range c.schemas.documents {
		names = append(names, document.doc.name)
	}
	return names
}

// ParseSchemaRootForTest parses a schema document and returns its root node.
func ParseSchemaRootForTest(data []byte, limits Limits) (*RawNode, error) {
	doc, err := parseRawSchemaDocument("test.xsd", "test.xsd", data, limits)
	if err != nil {
		return nil, err
	}
	return doc.root, nil
}

// FreezeCompilerRuntimeForTest freezes a compiler runtime for white-box tests.
func FreezeCompilerRuntimeForTest(c *Compiler) (*runtime.Schema, error) {
	return c.publishSchema()
}
