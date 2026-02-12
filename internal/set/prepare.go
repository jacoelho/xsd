package set

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/preprocessor"
	"github.com/jacoelho/xsd/internal/xmltree"
)

// Prepare loads and normalizes a schema for repeated runtime builds.
func Prepare(cfg PrepareConfig) (*PreparedSchema, error) {
	if cfg.FS == nil {
		return nil, fmt.Errorf("prepare schema: nil fs")
	}
	loader := preprocessor.NewLoader(preprocessor.Config{
		FS:                          cfg.FS,
		AllowMissingImportLocations: cfg.AllowMissingImportLocations,
		SchemaParseOptions:          cfg.SchemaParseOptions,
		DocumentPool:                xmltree.NewDocumentPool(),
	})
	parsed, err := loader.Load(cfg.Location)
	if err != nil {
		return nil, fmt.Errorf("load parsed schema: %w", err)
	}
	return PrepareParsedOwned(parsed)
}

// PrepareParsed normalizes a parsed schema by cloning it first.
func PrepareParsed(parsed *parser.Schema) (*PreparedSchema, error) {
	prepared, err := compiler.Prepare(parsed)
	if err != nil {
		return nil, err
	}
	return &PreparedSchema{prepared: prepared}, nil
}

// PrepareParsedOwned normalizes a parsed schema in place.
func PrepareParsedOwned(parsed *parser.Schema) (*PreparedSchema, error) {
	prepared, err := compiler.PrepareOwned(parsed)
	if err != nil {
		return nil, err
	}
	return &PreparedSchema{prepared: prepared}, nil
}
