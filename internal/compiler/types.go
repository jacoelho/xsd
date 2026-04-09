package compiler

import (
	"io/fs"
	"sync"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Root identifies one schema root document.
type Root struct {
	FS       fs.FS
	Location string
}

// LoadConfig configures schema load and normalization.
type LoadConfig struct {
	Roots                       []Root
	FS                          fs.FS
	Location                    string
	Resolver                    SchemaResolver
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// BuildConfig configures runtime compilation.
type BuildConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// Prepared stores normalized artifacts and lazy build state.
type Prepared struct {
	schema       *parser.Schema
	registry     *analysis.Registry
	refs         *analysis.ResolvedReferences
	complexTypes *complexplan.ComplexTypes
	prepared     *PreparedArtifacts
	prepErr      error
	buildOnce    sync.Once
}

// Schema returns the prepared schema graph.
func (p *Prepared) Schema() *parser.Schema {
	if p == nil {
		return nil
	}
	return p.schema
}

// Registry returns deterministic component IDs for the prepared schema.
func (p *Prepared) Registry() *analysis.Registry {
	if p == nil {
		return nil
	}
	return p.registry
}

// References returns the resolved reference index for the prepared schema.
func (p *Prepared) References() *analysis.ResolvedReferences {
	if p == nil {
		return nil
	}
	return p.refs
}

// ComplexTypes returns the prepared effective complex-type plan.
func (p *Prepared) ComplexTypes() *complexplan.ComplexTypes {
	if p == nil {
		return nil
	}
	return p.complexTypes
}
