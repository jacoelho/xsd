package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

// RuntimeProgram contains compiled runtime facet inputs for one validator.
type RuntimeProgram struct {
	Enums      runtime.EnumTable
	Facets     []runtime.FacetInstr
	Patterns   []runtime.Pattern
	Values     runtime.ValueBlob
	Normalized []byte
	Canonical  []byte
	Meta       runtime.ValidatorMeta
}

// RuntimeProgramSlice returns the facet instruction slice for a validator meta.
func RuntimeProgramSlice(meta runtime.ValidatorMeta, facets []runtime.FacetInstr) ([]runtime.FacetInstr, error) {
	if meta.Facets.Len == 0 {
		return nil, nil
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(facets) {
		return nil, fmt.Errorf("facet program out of range")
	}
	return facets[start:end], nil
}

// RuntimeProgramHasOp reports whether a validator facet program contains any requested op.
func RuntimeProgramHasOp(meta runtime.ValidatorMeta, facets []runtime.FacetInstr, ops ...runtime.FacetOp) (bool, error) {
	program, err := RuntimeProgramSlice(meta, facets)
	if err != nil {
		return false, err
	}
	for _, instr := range program {
		if slices.Contains(ops, instr.Op) {
			return true, nil
		}
	}
	return false, nil
}

// RuntimeProgramEnumIDs returns enumeration IDs referenced by facet instructions.
func RuntimeProgramEnumIDs(program []runtime.FacetInstr) []runtime.EnumID {
	if len(program) == 0 {
		return nil
	}
	out := make([]runtime.EnumID, 0, len(program))
	for _, instr := range program {
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func facetValueBytes(values runtime.ValueBlob, ref runtime.ValueRef) []byte {
	if !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start := int(ref.Off)
	end := start + int(ref.Len)
	if start < 0 || end < 0 || end > len(values.Blob) {
		return nil
	}
	return values.Blob[start:end]
}
