package compiler

import (
	"fmt"
	"iter"

	"github.com/jacoelho/xsd/internal/objects"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimeassemble"
)

// Build compiles prepared artifacts into an immutable runtime schema.
func (p *Prepared) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil || p.artifacts == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	prepared, err := p.ensureBuildArtifacts()
	if err != nil {
		return nil, err
	}
	return prepared.Build(runtimeassemble.BuildConfig{
		MaxDFAStates:   cfg.MaxDFAStates,
		MaxOccursLimit: cfg.MaxOccursLimit,
	})
}

// GlobalElementOrderSeq yields deterministic global element order.
func (p *Prepared) GlobalElementOrderSeq() iter.Seq[objects.QName] {
	return func(yield func(objects.QName) bool) {
		if p == nil || p.artifacts == nil {
			return
		}
		for item := range p.artifacts.GlobalElementOrderSeq() {
			if !yield(item) {
				return
			}
		}
	}
}

func (p *Prepared) ensureBuildArtifacts() (*runtimeassemble.PreparedArtifacts, error) {
	p.buildMu.Lock()
	defer p.buildMu.Unlock()

	p.buildOnce.Do(func() {
		p.prepared, p.prepErr = runtimeassemble.PrepareBuildArtifactsWithComplexTypePlan(
			p.artifacts.Schema(),
			p.artifacts.Registry(),
			p.artifacts.References(),
			p.artifacts.ComplexTypes(),
		)
	})
	if p.prepErr != nil {
		return nil, p.prepErr
	}
	return p.prepared, nil
}
