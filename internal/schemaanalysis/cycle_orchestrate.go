package schemaanalysis

import (
	"github.com/jacoelho/xsd/internal/parser"
)

type visitState uint8

const (
	stateUnseen visitState = iota
	stateVisiting
	stateDone
)

// DetectCycles validates that type derivation, group refs, attribute group refs,
// and substitution groups are acyclic.
func DetectCycles(schema *parser.Schema) error {
	if err := RequireResolved(schema); err != nil {
		return err
	}
	if err := validateSchemaInput(schema); err != nil {
		return err
	}

	if err := detectTypeCycles(schema); err != nil {
		return err
	}
	if err := detectGroupCycles(schema); err != nil {
		return err
	}
	if err := detectAttributeGroupCycles(schema); err != nil {
		return err
	}
	if err := detectSubstitutionGroupCycles(schema); err != nil {
		return err
	}
	return nil
}
