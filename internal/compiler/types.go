package compiler

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Root identifies one schema root document.
type Root struct {
	FS       fs.FS
	Resolver SchemaResolver
	Location string
}

// LoadConfig configures schema load and normalization.
type LoadConfig struct {
	Roots                       []Root
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// BuildConfig configures runtime compilation.
type BuildConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// ResolvedReferences stores resolved reference indexes consumed by runtime lowering.
type ResolvedReferences struct {
	ElementRefs   map[model.QName]analysis.ElemID
	AttributeRefs map[model.QName]analysis.AttrID
	GroupRefs     map[model.QName]model.QName
}

// Prepared stores normalized artifacts and lazy build state.
type Prepared struct {
	schema       *parser.Schema
	registry     *analysis.Registry
	refs         *ResolvedReferences
	complexTypes *complexplan.ComplexTypes
	build        preparedBuildState
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
func (p *Prepared) References() *ResolvedReferences {
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
