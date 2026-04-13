package xsd

import (
	"fmt"
	"io/fs"

	"github.com/jacoelho/xsd/internal/compiler"
)

// PreparedSchema stores prepared schema artifacts that can be built repeatedly.
type PreparedSchema struct {
	prepared *compiler.Prepared
}

// SourceSet owns schema sources and prepares/builds them through explicit phases.
type SourceSet struct {
	entries []compiler.Root
	source  sourceConfig
}

// NewSourceSet creates an empty source set.
func NewSourceSet(opts ...SourceOption) *SourceSet {
	set := &SourceSet{}
	applySourceOptions(&set.source, opts)
	return set
}

// WithOptions replaces source-set options.
func (s *SourceSet) WithOptions(opts ...SourceOption) *SourceSet {
	if s == nil {
		return nil
	}
	s.source = sourceConfig{}
	applySourceOptions(&s.source, opts)
	return s
}

// AddFS adds one schema root location from fsys.
func (s *SourceSet) AddFS(fsys fs.FS, location string) error {
	if s == nil {
		return fmt.Errorf("source set: nil set")
	}
	root, err := newCompileRoot(fsys, location)
	if err != nil {
		return err
	}
	s.entries = append(s.entries, root)
	return nil
}

// Prepare loads and resolves all added schema roots into reusable prepared artifacts.
func (s *SourceSet) Prepare() (*PreparedSchema, error) {
	if s == nil {
		return nil, fmt.Errorf("prepare source set: nil set")
	}
	req, err := newSourceCompileRequest(s.entries, s.source)
	if err != nil {
		return nil, fmt.Errorf("prepare source set: %w", err)
	}
	return preparePreparedSchema(req)
}

// Build compiles all added schema roots into an immutable runtime schema.
func (s *SourceSet) Build(opts ...BuildOption) (*Schema, error) {
	prepared, err := s.Prepare()
	if err != nil {
		return nil, err
	}
	schema, err := prepared.Build(opts...)
	if err != nil {
		return nil, fmt.Errorf("build source set: %w", err)
	}
	return schema, nil
}

// Build compiles prepared schema artifacts into an immutable runtime schema.
func (p *PreparedSchema) Build(opts ...BuildOption) (*Schema, error) {
	if p == nil {
		return nil, fmt.Errorf("build prepared schema: nil prepared schema")
	}
	return newBuildCompileRequest(opts).buildPrepared(p.prepared)
}
