package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/runtime"
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

type compilationPipeline struct {
	opts LoadOptions
	root string
	fsys fs.FS
}

type loadedArtifacts struct {
	loader *source.SchemaLoader
}

type parsedArtifacts struct {
	schema *parser.Schema
}

type preparedArtifacts struct {
	prepared *pipeline.PreparedSchema
}

func newCompilationPipeline(fsys fs.FS, root string, opts LoadOptions) *compilationPipeline {
	return &compilationPipeline{
		fsys: fsys,
		root: root,
		opts: opts,
	}
}

func (p *compilationPipeline) Run() (*runtime.Schema, error) {
	loaded, err := p.Load()
	if err != nil {
		return nil, err
	}
	parsed, err := p.Parse(loaded)
	if err != nil {
		return nil, err
	}
	prepared, err := p.Prepare(parsed)
	if err != nil {
		return nil, err
	}
	return p.Compile(prepared)
}

func (p *compilationPipeline) Load() (*loadedArtifacts, error) {
	if p == nil || p.fsys == nil {
		return nil, fmt.Errorf("compile schema: nil fs")
	}
	loader := source.NewLoader(source.Config{
		FS:                          p.fsys,
		AllowMissingImportLocations: p.opts.AllowMissingImportLocations,
		SchemaParseOptions: buildXMLParseOptions(
			p.opts.SchemaMaxDepth,
			p.opts.SchemaMaxAttrs,
			p.opts.SchemaMaxTokenSize,
			p.opts.SchemaMaxQNameInternEntries,
		),
	})
	return &loadedArtifacts{loader: loader}, nil
}

func (p *compilationPipeline) Parse(loaded *loadedArtifacts) (*parsedArtifacts, error) {
	if loaded == nil || loaded.loader == nil {
		return nil, fmt.Errorf("compile schema %s: nil schema loader", p.root)
	}
	parsed, err := loaded.loader.Load(p.root)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", p.root, err)
	}
	return &parsedArtifacts{schema: parsed}, nil
}

func (p *compilationPipeline) Prepare(parsed *parsedArtifacts) (*preparedArtifacts, error) {
	if parsed == nil || parsed.schema == nil {
		return nil, fmt.Errorf("compile schema %s: nil parsed schema", p.root)
	}
	validated, err := pipeline.Validate(parsed.schema)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", p.root, err)
	}
	prepared, err := pipeline.Transform(validated)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", p.root, err)
	}
	return &preparedArtifacts{prepared: prepared}, nil
}

func (p *compilationPipeline) Compile(prepared *preparedArtifacts) (*runtime.Schema, error) {
	if prepared == nil || prepared.prepared == nil {
		return nil, fmt.Errorf("compile schema %s: nil prepared schema", p.root)
	}
	rt, err := prepared.prepared.BuildRuntime(buildConfigFrom(p.opts))
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", p.root, err)
	}
	return rt, nil
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
	p := newCompilationPipeline(fsys, root, opts)
	rt, err := p.Run()
	if err != nil {
		return nil, err
	}
	return newEngine(rt, buildXMLParseOptions(
		opts.InstanceMaxDepth,
		opts.InstanceMaxAttrs,
		opts.InstanceMaxTokenSize,
		opts.InstanceMaxQNameInternEntries,
	)...), nil
}

func buildConfigFrom(opts LoadOptions) pipeline.CompileConfig {
	return pipeline.CompileConfig{
		Limits: contentmodel.Limits{
			MaxDFAStates: opts.MaxDFAStates,
		},
		MaxOccursLimit: opts.MaxOccursLimit,
	}
}
