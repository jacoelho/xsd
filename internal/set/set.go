package set

import (
	"fmt"
	"iter"

	"github.com/jacoelho/xsd/internal/objects"
	"github.com/jacoelho/xsd/internal/runtime"
)

// Set owns one prepared schema graph and runtime compilation entrypoints.
type Set struct {
	prepared *PreparedSchema
}

// NewSet creates an empty schema set.
func NewSet() *Set {
	return &Set{}
}

// Prepare loads and normalizes a schema graph into the set.
func (s *Set) Prepare(cfg PrepareConfig) error {
	if s == nil {
		return fmt.Errorf("schema set: nil set")
	}
	prepared, err := Prepare(cfg)
	if err != nil {
		return err
	}
	s.prepared = prepared
	return nil
}

// BuildRuntime compiles the prepared schema into immutable runtime form.
func (s *Set) BuildRuntime(cfg CompileConfig) (*runtime.Schema, error) {
	if s == nil || s.prepared == nil {
		return nil, fmt.Errorf("schema set: schema not prepared")
	}
	return s.prepared.BuildRuntime(cfg)
}

// IsPrepared reports whether the set currently owns prepared artifacts.
func (s *Set) IsPrepared() bool {
	return s != nil && s.prepared != nil
}

// GlobalElementOrderSeq yields deterministic global element order.
func (s *Set) GlobalElementOrderSeq() iter.Seq[objects.QName] {
	return func(yield func(objects.QName) bool) {
		if s == nil || s.prepared == nil {
			return
		}
		for item := range s.prepared.GlobalElementOrderSeq() {
			if !yield(item) {
				return
			}
		}
	}
}

// Prepared returns the current prepared schema.
func (s *Set) Prepared() *PreparedSchema {
	if s == nil {
		return nil
	}
	return s.prepared
}
