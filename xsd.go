package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"sync"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	engine *engine
}

// QName is a public qualified name with namespace and local part.
type QName = xmlstream.QName

// PreparedSchema stores immutable, precompiled schema artifacts.
type PreparedSchema struct {
	prepared *pipeline.PreparedSchema
	runtime  resolvedRuntimeOptions

	buildCacheMu sync.RWMutex
	buildCache   map[runtimeBuildCacheKey]*runtime.Schema
	buildOrder   []runtimeBuildCacheKey
}

type runtimeBuildCacheKey struct {
	maxDFAStates   uint32
	maxOccursLimit uint32
}

const maxPreparedRuntimeBuildCacheEntries = 16

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
	key := runtimeBuildKey(opts)
	p.buildCacheMu.RLock()
	rt, ok := p.buildCache[key]
	p.buildCacheMu.RUnlock()
	if !ok {
		p.buildCacheMu.Lock()
		rt, ok = p.buildCache[key]
		if !ok {
			var err error
			rt, err = p.prepared.BuildRuntime(buildCompileConfig(opts))
			if err != nil {
				p.buildCacheMu.Unlock()
				return nil, fmt.Errorf("build schema: %w", err)
			}
			if p.buildCache == nil {
				p.buildCache = make(map[runtimeBuildCacheKey]*runtime.Schema)
			}
			if maxPreparedRuntimeBuildCacheEntries > 0 && len(p.buildCache) >= maxPreparedRuntimeBuildCacheEntries {
				oldest := p.buildOrder[0]
				p.buildOrder = p.buildOrder[1:]
				delete(p.buildCache, oldest)
			}
			p.buildCache[key] = rt
			p.buildOrder = append(p.buildOrder, key)
		}
		p.buildCacheMu.Unlock()
	}
	return &Schema{engine: newEngine(rt, opts.instanceParseOptions...)}, nil
}

func runtimeBuildKey(opts resolvedRuntimeOptions) runtimeBuildCacheKey {
	return runtimeBuildCacheKey{
		maxDFAStates:   opts.maxDFAStates,
		maxOccursLimit: opts.maxOccursLimit,
	}
}

func (p *PreparedSchema) runtimeBuildCacheLen() int {
	if p == nil {
		return 0
	}
	p.buildCacheMu.RLock()
	n := len(p.buildCache)
	p.buildCacheMu.RUnlock()
	return n
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
	return s.validateReader(r, "")
}

// ValidateFile validates an XML file against the schema.
func (s *Schema) ValidateFile(path string) (err error) {
	if s == nil || s.engine == nil {
		return schemaNotLoadedError()
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open xml file %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close xml file %s: %w", path, closeErr)
		}
	}()

	err = s.validateReader(f, path)
	return err
}

func (s *Schema) validateReader(r io.Reader, document string) error {
	if s == nil || s.engine == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
	}
	if r == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
	}
	return s.engine.validateDocument(r, document)
}

func buildCompileConfig(opts resolvedRuntimeOptions) pipeline.CompileConfig {
	return pipeline.CompileConfig{
		Limits: contentmodel.Limits{
			MaxDFAStates: opts.maxDFAStates,
		},
		MaxOccursLimit: opts.maxOccursLimit,
	}
}
