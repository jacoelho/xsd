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

type compilationPipeline struct {
	fsys                        fs.FS
	root                        string
	schemaLimits                xmlParseLimits
	allowMissingImportLocations bool
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

func newCompilationPipeline(fsys fs.FS, root string, opts LoadOptions) (*compilationPipeline, resolvedRuntimeOptions, error) {
	resolved, runtimeOpts, err := opts.withDefaults()
	if err != nil {
		return nil, resolvedRuntimeOptions{}, err
	}
	return newCompilationPipelineResolved(fsys, root, resolved), runtimeOpts, nil
}

func newCompilationPipelineResolved(fsys fs.FS, root string, opts resolvedLoadOptions) *compilationPipeline {
	return &compilationPipeline{
		schemaLimits:                opts.schemaLimits,
		fsys:                        fsys,
		root:                        root,
		allowMissingImportLocations: opts.allowMissingImportLocations,
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
		AllowMissingImportLocations: p.allowMissingImportLocations,
		SchemaParseOptions:          p.schemaLimits.options(),
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
	return p.CompileWithRuntime(prepared, resolvedRuntimeOptions{})
}

func (p *compilationPipeline) CompileWithRuntime(prepared *preparedArtifacts, runtimeOpts resolvedRuntimeOptions) (*runtime.Schema, error) {
	if prepared == nil || prepared.prepared == nil {
		return nil, fmt.Errorf("compile schema %s: nil prepared schema", p.root)
	}
	rt, err := prepared.prepared.BuildRuntime(buildCompileConfig(runtimeOpts))
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", p.root, err)
	}
	return rt, nil
}

// Prepare loads and validates a schema, returning reusable prepared artifacts.
func Prepare(fsys fs.FS, location string) (*PreparedSchema, error) {
	return PrepareWithOptions(fsys, location, NewLoadOptions())
}

// PrepareWithOptions loads and validates a schema with explicit load options.
func PrepareWithOptions(fsys fs.FS, location string, opts LoadOptions) (*PreparedSchema, error) {
	p, runtimeOpts, err := newCompilationPipeline(fsys, location, opts)
	if err != nil {
		return nil, fmt.Errorf("prepare schema %s: %w", location, err)
	}
	loaded, err := p.Load()
	if err != nil {
		return nil, fmt.Errorf("prepare schema %s: %w", location, err)
	}
	parsed, err := p.Parse(loaded)
	if err != nil {
		return nil, fmt.Errorf("prepare schema %s: %w", location, err)
	}
	prepared, err := p.Prepare(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema %s: %w", location, err)
	}
	return &PreparedSchema{
		prepared: prepared.prepared,
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

// GlobalElementOrder returns deterministic global element order from preparation.
func (p *PreparedSchema) GlobalElementOrder() []QName {
	if p == nil || p.prepared == nil {
		return nil
	}
	internalOrder := p.prepared.GlobalElementOrder()
	if len(internalOrder) == 0 {
		return nil
	}
	order := make([]QName, len(internalOrder))
	for i, item := range internalOrder {
		order[i] = QName{
			Namespace: item.Namespace.String(),
			Local:     item.Local,
		}
	}
	return order
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
