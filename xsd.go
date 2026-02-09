package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/source"
)

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	engine *engine
}

// QName is a public qualified name with namespace and local part.
type QName struct {
	Namespace string
	Local     string
}

// PreparedSchema stores immutable, precompiled schema artifacts.
type PreparedSchema struct {
	prepared *pipeline.PreparedSchema
	runtime  resolvedRuntimeOptions
}

func prepareSchema(fsys fs.FS, location string, opts LoadOptions) (*pipeline.PreparedSchema, resolvedRuntimeOptions, error) {
	resolvedLoad, runtimeOpts, err := opts.withDefaults()
	if err != nil {
		return nil, resolvedRuntimeOptions{}, fmt.Errorf("schema options: %w", err)
	}
	if fsys == nil {
		return nil, resolvedRuntimeOptions{}, fmt.Errorf("nil fs")
	}

	loader := source.NewLoader(source.Config{
		FS:                          fsys,
		AllowMissingImportLocations: resolvedLoad.allowMissingImportLocations,
		SchemaParseOptions:          resolvedLoad.schemaLimits.options(),
	})
	parsed, err := loader.Load(location)
	if err != nil {
		return nil, resolvedRuntimeOptions{}, fmt.Errorf("load parsed schema: %w", err)
	}

	validated, err := pipeline.Validate(parsed)
	if err != nil {
		return nil, resolvedRuntimeOptions{}, fmt.Errorf("validate schema: %w", err)
	}
	prepared, err := pipeline.Transform(validated)
	if err != nil {
		return nil, resolvedRuntimeOptions{}, fmt.Errorf("transform schema: %w", err)
	}
	return prepared, runtimeOpts, nil
}

// Prepare loads and validates a schema, returning reusable prepared artifacts.
func Prepare(fsys fs.FS, location string) (*PreparedSchema, error) {
	return PrepareWithOptions(fsys, location, NewLoadOptions())
}

// PrepareWithOptions loads and validates a schema with explicit load options.
func PrepareWithOptions(fsys fs.FS, location string, opts LoadOptions) (*PreparedSchema, error) {
	prepared, runtimeOpts, err := prepareSchema(fsys, location, opts)
	if err != nil {
		return nil, fmt.Errorf("prepare schema %s: %w", location, err)
	}
	return &PreparedSchema{
		prepared: prepared,
		runtime:  runtimeOpts,
	}, nil
}

// Build compiles prepared artifacts into a runtime validator schema.
func (p *PreparedSchema) Build() (*Schema, error) {
	if p == nil {
		return nil, fmt.Errorf("build schema: prepared schema is nil")
	}
	return p.buildWithResolvedRuntime(p.runtime)
}

// BuildWithOptions compiles prepared artifacts using explicit runtime options.
func (p *PreparedSchema) BuildWithOptions(opts RuntimeOptions) (*Schema, error) {
	if p == nil {
		return nil, fmt.Errorf("build schema: prepared schema is nil")
	}
	resolved, err := opts.withDefaults()
	if err != nil {
		return nil, fmt.Errorf("build schema: %w", err)
	}
	return p.buildWithResolvedRuntime(resolved)
}

// GlobalElementOrderSeq yields deterministic global element order from preparation.
func (p *PreparedSchema) GlobalElementOrderSeq() iter.Seq[QName] {
	return func(yield func(QName) bool) {
		if p == nil || p.prepared == nil {
			return
		}
		for item := range p.prepared.GlobalElementOrderSeq() {
			if !yield(QName{
				Namespace: item.Namespace.String(),
				Local:     item.Local,
			}) {
				return
			}
		}
	}
}

func (p *PreparedSchema) buildWithResolvedRuntime(opts resolvedRuntimeOptions) (*Schema, error) {
	if p == nil || p.prepared == nil {
		return nil, fmt.Errorf("build schema: prepared schema is nil")
	}
	rt, err := p.prepared.BuildRuntime(buildCompileConfig(opts))
	if err != nil {
		return nil, fmt.Errorf("build schema: %w", err)
	}
	return &Schema{engine: newEngine(rt, opts.instanceParseOptions...)}, nil
}

// Load loads and compiles a schema from the given filesystem and location.
func Load(fsys fs.FS, location string) (*Schema, error) {
	return LoadWithOptions(fsys, location, NewLoadOptions())
}

// LoadWithOptions loads and compiles a schema with explicit configuration.
func LoadWithOptions(fsys fs.FS, location string, opts LoadOptions) (*Schema, error) {
	prepared, err := PrepareWithOptions(fsys, location, opts)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	schema, err := prepared.Build()
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	return schema, nil
}

// LoadFile loads and compiles a schema from a file path.
func LoadFile(path string) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return LoadWithOptions(os.DirFS(dir), base, NewLoadOptions())
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

func buildCompileConfig(opts resolvedRuntimeOptions) pipeline.CompileConfig {
	return pipeline.CompileConfig{
		Limits: contentmodel.Limits{
			MaxDFAStates: opts.maxDFAStates,
		},
		MaxOccursLimit: opts.maxOccursLimit,
	}
}
