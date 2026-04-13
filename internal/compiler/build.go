package compiler

import (
	"fmt"
	"iter"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validatorbuild"
)

// Build compiles prepared artifacts into an immutable runtime schema.
func (p *Prepared) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil || p.schema == nil || p.registry == nil || p.refs == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	validators, err := p.ensureValidators()
	if err != nil {
		return nil, err
	}
	return Build(p.schema, p.registry, p.refs, validators, Config(cfg))
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

func (p *Prepared) ensureValidators() (*validatorbuild.ValidatorArtifacts, error) {
	p.buildOnce.Do(func() {
		validators, err := prepareValidators(p.schema, p.registry, p.refs, p.complexTypes)
		if err != nil {
			p.prepErr = err
			return
		}
		p.validators = validators
	})
	if p.prepErr != nil {
		return nil, p.prepErr
	}
	return p.validators, nil
}
