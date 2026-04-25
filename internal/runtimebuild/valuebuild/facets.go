package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (c *artifactCompiler) compileFacetProgram(spec schemair.SimpleTypeSpec) (runtime.FacetProgramRef, error) {
	facets := spec.Facets
	if spec.Builtin && spec.Variety == schemair.TypeVarietyList {
		facets = append([]schemair.FacetSpec{{Kind: schemair.FacetMinLength, Name: "minLength", IntValue: 1}}, facets...)
	}
	if len(facets) == 0 {
		return runtime.FacetProgramRef{}, nil
	}
	start := len(c.facets)
	for _, facet := range facets {
		switch facet.Kind {
		case schemair.FacetPattern:
			values := make([]string, 0, len(facet.Values))
			for _, value := range facet.Values {
				values = append(values, value.Lexical)
			}
			patternID, err := c.addPatternSet(values)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(patternID)})
		case schemair.FacetEnumeration:
			enumID, err := c.compileEnumeration(spec, facet)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FEnum, Arg0: uint32(enumID)})
		case schemair.FacetLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FLength, Arg0: facet.IntValue})
		case schemair.FacetMinLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMinLength, Arg0: facet.IntValue})
		case schemair.FacetMaxLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMaxLength, Arg0: facet.IntValue})
		case schemair.FacetTotalDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FTotalDigits, Arg0: facet.IntValue})
		case schemair.FacetFractionDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FFractionDigits, Arg0: facet.IntValue})
		case schemair.FacetMinInclusive, schemair.FacetMaxInclusive, schemair.FacetMinExclusive, schemair.FacetMaxExclusive:
			op, ok := rangeFacetOp(facet.Kind)
			if !ok {
				continue
			}
			normalized := c.normalizeLexical(facet.Value, spec)
			canon, err := c.canonicalizeNormalized(facet.Value, normalized, spec, nil, canonicalizeGeneral)
			if err != nil {
				return runtime.FacetProgramRef{}, fmt.Errorf("%s: %w", facet.Name, err)
			}
			ref := c.values.addWithHash(canon, runtime.HashBytes(canon))
			c.facets = append(c.facets, runtime.FacetInstr{Op: op, Arg0: ref.Off, Arg1: ref.Len})
		default:
			continue
		}
	}
	return runtime.FacetProgramRef{Off: uint32(start), Len: uint32(len(c.facets) - start)}, nil
}

func (c *artifactCompiler) compileEnumeration(spec schemair.SimpleTypeSpec, facet schemair.FacetSpec) (runtime.EnumID, error) {
	if len(facet.Values) == 0 {
		return 0, nil
	}
	keys := make([]runtime.ValueKey, 0, len(facet.Values))
	for _, value := range facet.Values {
		normalized := c.normalizeLexical(value.Lexical, spec)
		enumKeys, err := c.valueKeysForNormalized(value.Lexical, normalized, spec, value.Context)
		if err != nil {
			return 0, err
		}
		keys = append(keys, enumKeys...)
	}
	return c.enums.add(keys), nil
}

func rangeFacetOp(kind schemair.FacetKind) (runtime.FacetOp, bool) {
	switch kind {
	case schemair.FacetMinInclusive:
		return runtime.FMinInclusive, true
	case schemair.FacetMaxInclusive:
		return runtime.FMaxInclusive, true
	case schemair.FacetMinExclusive:
		return runtime.FMinExclusive, true
	case schemair.FacetMaxExclusive:
		return runtime.FMaxExclusive, true
	default:
		return 0, false
	}
}
