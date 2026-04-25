package compiler

import (
	"fmt"
	"iter"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimebuild"
	"github.com/jacoelho/xsd/internal/schemaast"
)

// Build compiles prepared artifacts into an immutable runtime schema.
func (p *Prepared) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil || p.ir == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return runtimebuild.Build(runtimebuild.Input{
		Schema: p.ir,
		Config: runtimebuild.Config{
			MaxDFAStates:   cfg.MaxDFAStates,
			MaxOccursLimit: cfg.MaxOccursLimit,
		},
	})
}

// GlobalElementOrderSeq yields deterministic global element order.
func (p *Prepared) GlobalElementOrderSeq() iter.Seq[schemaast.QName] {
	return func(yield func(schemaast.QName) bool) {
		if p == nil || p.ir == nil {
			return
		}
		for _, entry := range p.ir.GlobalIndexes.Elements {
			qname := schemaast.QName{Namespace: entry.Name.Namespace, Local: entry.Name.Local}
			if yield(qname) {
				continue
			}
			return
		}
	}
}
