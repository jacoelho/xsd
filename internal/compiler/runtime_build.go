package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/validatorbuild"
)

func prepareValidators(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	complexTypes *complexplan.ComplexTypes,
) (*validatorbuild.ValidatorArtifacts, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	if complexTypes == nil {
		return nil, fmt.Errorf("runtime build: complex types are nil")
	}
	return validatorbuild.Compile(sch, reg, complexTypes)
}
