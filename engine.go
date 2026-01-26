package xsd

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/models"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimebuild"
	"github.com/jacoelho/xsd/internal/validator"
)

// Engine compiles a schema once and validates many documents efficiently.
// It is safe for concurrent use by multiple goroutines.
type Engine struct {
	rt   *runtime.Schema
	pool sync.Pool
}

// Session holds per-document state for validation.
// Sessions are not safe for concurrent use.
type Session struct {
	engine  *Engine
	session *validator.Session
}

// CompileOption configures schema compilation.
type CompileOption interface{ apply(*compileOptions) }

// ValidateOption configures validation.
type ValidateOption interface{ apply(*validateOptions) }

// CompileLimits constrain compilation behavior.
type CompileLimits struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

type compileOptions struct {
	fsys                        fs.FS
	resolver                    Resolver
	baseSystemID                string
	allowMissingImportLocations bool
	limits                      CompileLimits
}

type validateOptions struct {
	entities map[string]struct{}
}

type compileOptionFunc func(*compileOptions)

func (f compileOptionFunc) apply(cfg *compileOptions) {
	if cfg == nil {
		return
	}
	f(cfg)
}

type validateOptionFunc func(*validateOptions)

func (f validateOptionFunc) apply(cfg *validateOptions) {
	if cfg == nil {
		return
	}
	f(cfg)
}

// WithResolver sets a custom schema resolver.
func WithResolver(r Resolver) CompileOption {
	return compileOptionFunc(func(cfg *compileOptions) {
		cfg.resolver = r
	})
}

// WithFS overrides the filesystem used for schema loading.
func WithFS(fsys fs.FS) CompileOption {
	return compileOptionFunc(func(cfg *compileOptions) {
		cfg.fsys = fsys
	})
}

// WithBaseSystemID sets the base system ID for reader-based compilation.
func WithBaseSystemID(base string) CompileOption {
	return compileOptionFunc(func(cfg *compileOptions) {
		cfg.baseSystemID = base
	})
}

// WithAllowMissingImportLocations controls import-without-location behavior.
func WithAllowMissingImportLocations(b bool) CompileOption {
	return compileOptionFunc(func(cfg *compileOptions) {
		cfg.allowMissingImportLocations = b
	})
}

// WithCompileLimits sets compilation limits.
func WithCompileLimits(l CompileLimits) CompileOption {
	return compileOptionFunc(func(cfg *compileOptions) {
		cfg.limits = l
	})
}

// WithEntities supplies declared ENTITY/ENTITIES names for validation.
func WithEntities(entities map[string]struct{}) ValidateOption {
	return validateOptionFunc(func(cfg *validateOptions) {
		cfg.entities = entities
	})
}

// CompileFS compiles a schema from the given filesystem and root path.
func CompileFS(fsys fs.FS, root string, opts ...CompileOption) (*Engine, error) {
	cfg := applyCompileOptions(opts)
	if cfg.fsys != nil {
		fsys = cfg.fsys
	}
	if fsys == nil && cfg.resolver == nil {
		return nil, fmt.Errorf("compile schema: nil fs")
	}

	l := loader.NewLoader(loader.Config{
		FS:                          fsys,
		Resolver:                    cfg.resolver,
		AllowMissingImportLocations: cfg.allowMissingImportLocations,
	})
	parsed, err := l.Load(root)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", root, err)
	}
	rt, err := runtimebuild.BuildSchema(parsed, buildConfigFrom(cfg))
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", root, err)
	}
	return newEngine(rt), nil
}

// CompileSchema compiles a schema from an io.Reader.
func CompileSchema(r io.Reader, opts ...CompileOption) (*Engine, error) {
	if r == nil {
		return nil, fmt.Errorf("compile schema: nil reader")
	}
	cfg := applyCompileOptions(opts)
	baseSystemID := cfg.baseSystemID
	if baseSystemID == "" {
		baseSystemID = "schema.xsd"
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	l := loader.NewLoader(loader.Config{
		FS:                          cfg.fsys,
		Resolver:                    cfg.resolver,
		AllowMissingImportLocations: cfg.allowMissingImportLocations,
	})

	parsed, err := l.LoadResolved(io.NopCloser(bytes.NewReader(data)), baseSystemID)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", baseSystemID, err)
	}
	rt, err := runtimebuild.BuildSchema(parsed, buildConfigFrom(cfg))
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", baseSystemID, err)
	}

	return newEngine(rt), nil
}

// Validate validates a document using a pooled session.
func (e *Engine) Validate(r io.Reader, opts ...ValidateOption) error {
	if e == nil || e.rt == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return nilReaderError()
	}

	cfg := applyValidateOptions(opts)
	session := e.acquire()
	err := session.Validate(r, cfg.entities)
	e.release(session)
	return err
}

// NewSession returns a new, unpooled session bound to this engine.
func (e *Engine) NewSession() *Session {
	if e == nil {
		return nil
	}
	return &Session{
		engine:  e,
		session: validator.NewSession(e.rt),
	}
}

// Validate validates a document using this session.
func (s *Session) Validate(r io.Reader, opts ...ValidateOption) error {
	cfg := applyValidateOptions(opts)
	return s.ValidateWithEntities(r, cfg.entities)
}

// ValidateWithEntities validates a document with declared ENTITY/ENTITIES names.
func (s *Session) ValidateWithEntities(r io.Reader, entities map[string]struct{}) error {
	if s == nil || s.engine == nil || s.engine.rt == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return nilReaderError()
	}
	if s.session == nil {
		s.session = validator.NewSession(s.engine.rt)
	}
	return s.session.Validate(r, entities)
}

// Reset clears per-document session state.
func (s *Session) Reset() {
	if s == nil || s.session == nil {
		return
	}
	s.session.Reset()
}

func newEngine(rt *runtime.Schema) *Engine {
	e := &Engine{
		rt: rt,
	}
	e.pool.New = func() any {
		return validator.NewSession(rt)
	}
	return e
}

func (e *Engine) acquire() *validator.Session {
	if e == nil {
		return nil
	}
	if v := e.pool.Get(); v != nil {
		session := v.(*validator.Session)
		return session
	}
	return validator.NewSession(e.rt)
}

func (e *Engine) release(s *validator.Session) {
	if e == nil || s == nil {
		return
	}
	s.Reset()
	e.pool.Put(s)
}

func schemaNotLoadedError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
}

func nilReaderError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
}

func applyCompileOptions(opts []CompileOption) compileOptions {
	var cfg compileOptions
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}
	return cfg
}

func buildConfigFrom(cfg compileOptions) runtimebuild.BuildConfig {
	limits := cfg.limits
	if limits.MaxDFAStates == 0 {
		limits.MaxDFAStates = 4096
	}
	if limits.MaxOccursLimit == 0 {
		limits.MaxOccursLimit = 1_000_000
	}
	return runtimebuild.BuildConfig{
		Limits: models.Limits{
			MaxDFAStates: limits.MaxDFAStates,
		},
		MaxOccursLimit: limits.MaxOccursLimit,
	}
}

func applyValidateOptions(opts []ValidateOption) validateOptions {
	var cfg validateOptions
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}
	return cfg
}
