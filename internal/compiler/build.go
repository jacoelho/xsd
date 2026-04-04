package compiler

import (
	"fmt"
	"iter"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

// Build compiles prepared artifacts into an immutable runtime schema.
func (p *Prepared) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil || p.schema == nil || p.registry == nil || p.refs == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	prepared, err := p.ensureBuildArtifacts()
	if err != nil {
		return nil, err
	}
	return prepared.Build(cfg)
}

// GlobalElementOrderSeq yields deterministic global element order.
func (p *Prepared) GlobalElementOrderSeq() iter.Seq[model.QName] {
	return func(yield func(model.QName) bool) {
		if p == nil || p.registry == nil {
			return
		}
		for _, entry := range p.registry.ElementOrder {
			if !entry.Global {
				continue
			}
			if yield(entry.QName) {
				continue
			}
			return
		}
	}
}

func (p *Prepared) ensureBuildArtifacts() (*PreparedArtifacts, error) {
	p.buildOnce.Do(func() {
		prepared, err := prepareBuildArtifactsFromSemantics(p.schema, p.registry, p.refs, p.semantics)
		if err != nil {
			p.prepErr = err
			return
		}
		p.prepared = prepared
	})
	if p.prepErr != nil {
		return nil, p.prepErr
	}
	return p.prepared, nil
}
