package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/runtimecompile"
	"github.com/jacoelho/xsd/internal/source"
)

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	engine *engine
}

// LoadOptions configures schema loading and compilation.
type LoadOptions struct {
	AllowMissingImportLocations   bool
	MaxDFAStates                  uint32
	MaxOccursLimit                uint32
	SchemaMaxDepth                int
	SchemaMaxAttrs                int
	SchemaMaxTokenSize            int
	SchemaMaxQNameInternEntries   int
	InstanceMaxDepth              int
	InstanceMaxAttrs              int
	InstanceMaxTokenSize          int
	InstanceMaxQNameInternEntries int
}

// Load loads and compiles a schema from the given filesystem and location.
func Load(fsys fs.FS, location string) (*Schema, error) {
	return LoadWithOptions(fsys, location, LoadOptions{})
}

// LoadWithOptions loads and compiles a schema with explicit configuration.
func LoadWithOptions(fsys fs.FS, location string, opts LoadOptions) (*Schema, error) {
	engine, err := compileFS(fsys, location, opts)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	return &Schema{engine: engine}, nil
}

// LoadFile loads and compiles a schema from a file path.
func LoadFile(path string) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return LoadWithOptions(os.DirFS(dir), base, LoadOptions{})
}

// Validate validates a document against the schema.
func (s *Schema) Validate(r io.Reader) error {
	if s == nil || s.engine == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
	}
	if r == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
	}
	return s.engine.validate(r)
}

// ValidateFile validates an XML file against the schema.
func (s *Schema) ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open xml file %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close xml file %s: %w", path, closeErr)
		}
	}()

	return s.engine.validateWithDocument(f, path)
}

func compileFS(fsys fs.FS, root string, opts LoadOptions) (*engine, error) {
	if fsys == nil {
		return nil, fmt.Errorf("compile schema: nil fs")
	}

	l := source.NewLoader(source.Config{
		FS:                          fsys,
		AllowMissingImportLocations: opts.AllowMissingImportLocations,
		SchemaParseOptions: buildXMLParseOptions(
			opts.SchemaMaxDepth,
			opts.SchemaMaxAttrs,
			opts.SchemaMaxTokenSize,
			opts.SchemaMaxQNameInternEntries,
		),
	})
	parsed, err := l.LoadParsed(root)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", root, err)
	}
	prepared, err := pipeline.Prepare(parsed)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", root, err)
	}
	rt, err := runtimecompile.BuildPrepared(prepared, buildConfigFrom(opts))
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", root, err)
	}
	return newEngine(rt, buildXMLParseOptions(
		opts.InstanceMaxDepth,
		opts.InstanceMaxAttrs,
		opts.InstanceMaxTokenSize,
		opts.InstanceMaxQNameInternEntries,
	)...), nil
}

func buildConfigFrom(opts LoadOptions) runtimecompile.BuildConfig {
	return runtimecompile.BuildConfig{
		Limits: contentmodel.Limits{
			MaxDFAStates: opts.MaxDFAStates,
		},
		MaxOccursLimit: opts.MaxOccursLimit,
	}
}
