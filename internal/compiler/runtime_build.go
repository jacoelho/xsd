package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/validatorbuild"
)

func buildValidatorArtifacts(prepared *Prepared) (*validatorbuild.ValidatorArtifacts, error) {
	if prepared == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	if err := validateBuildInputs(prepared.schema, prepared.registry, prepared.refs); err != nil {
		return nil, err
	}
	if prepared.complexTypes == nil {
		return nil, fmt.Errorf("runtime build: complex types are nil")
	}
	return validatorbuild.Compile(prepared.schema, prepared.registry, prepared.complexTypes)
}
