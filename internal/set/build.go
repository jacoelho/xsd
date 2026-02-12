package set

import (
	"fmt"
	"iter"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/objects"
	"github.com/jacoelho/xsd/internal/runtime"
)

// BuildRuntime compiles prepared artifacts into an immutable runtime schema.
func (p *PreparedSchema) BuildRuntime(cfg CompileConfig) (*runtime.Schema, error) {
	if p == nil || p.prepared == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return p.prepared.Build(toBuildConfig(cfg))
}

func toBuildConfig(cfg CompileConfig) compiler.BuildConfig {
	return compiler.BuildConfig{
		MaxDFAStates:   cfg.MaxDFAStates,
		MaxOccursLimit: cfg.MaxOccursLimit,
	}
}

// GlobalElementOrderSeq yields deterministic global element order.
func (p *PreparedSchema) GlobalElementOrderSeq() iter.Seq[objects.QName] {
	return func(yield func(objects.QName) bool) {
		if p == nil || p.prepared == nil {
			return
		}
		for item := range p.prepared.GlobalElementOrderSeq() {
			if !yield(item) {
				return
			}
		}
	}
}
