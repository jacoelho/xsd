package compiler

import (
	"sync"

	"github.com/jacoelho/xsd/internal/validatorbuild"
)

type preparedBuildState struct {
	once       sync.Once
	validators *validatorbuild.ValidatorArtifacts
	err        error
}

func (s *preparedBuildState) ensureValidators(prepared *Prepared) (*validatorbuild.ValidatorArtifacts, error) {
	if s == nil {
		return nil, nil
	}
	s.once.Do(func() {
		s.validators, s.err = buildValidatorArtifacts(prepared)
	})
	if s.err != nil {
		return nil, s.err
	}
	return s.validators, nil
}
