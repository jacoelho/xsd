package compile

import (
	"bytes"
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
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
	return &c.rt
}

// NameInternerIsZeroForTest reports whether the compiler name interner was cleared.
func (c *compiler) NameInternerIsZeroForTest() bool {
	return c.names.IsZero()
}

// DocumentNamesForTest returns loaded schema document names in compiler order.
func (c *compiler) DocumentNamesForTest() []string {
	names := make([]string, 0, len(c.documents))
	for _, document := range c.documents {
		if document.indexDeclarations {
			names = append(names, document.doc.name)
		}
	}
	return names
}

// ValidateCompiledModelDerivedForTest validates a compiled model against its source.
func ValidateCompiledModelDerivedForTest(rt *runtime.SchemaBuild, id runtime.ContentModelID, model runtime.CompiledModel) error {
	return ValidateCompiledModelDerived(&rt.Names, rt, id, model)
}

// ParseSchemaRootForTest parses a schema document and returns its root node.
func ParseSchemaRootForTest(data []byte, limits Limits) (*RawNode, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	state := schemaParseState{
		dec:    dec,
		limits: limits,
		nsStack: []map[string]string{{
			vocab.XMLPrefix: vocab.XMLNamespaceURI,
		}},
	}
	if err := state.parse(); err != nil {
		return nil, err
	}
	return state.root, nil
}

// FreezeCompilerRuntimeForTest freezes a compiler runtime for white-box tests.
func FreezeCompilerRuntimeForTest(c *Compiler) (*runtime.Schema, error) {
	return publishCompilerRuntime(c)
}
