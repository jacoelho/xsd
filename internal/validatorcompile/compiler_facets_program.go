package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *compiler) compileFacetProgram(st *model.SimpleType, facets, partial []model.Facet) (runtime.FacetProgramRef, error) {
	if len(facets) == 0 {
		return runtime.FacetProgramRef{}, nil
	}
	start := len(c.facets)
	for _, facet := range facets {
		switch f := facet.(type) {
		case *model.Pattern:
			pid, err := c.addPattern(f)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(pid)})
		case *model.PatternSet:
			pid, err := c.addPatternSet(f)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(pid)})
		case *model.Enumeration:
			enumID, err := c.compileEnumeration(f, st, partial)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FEnum, Arg0: uint32(enumID)})
		case *model.Length:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FLength, Arg0: uint32(f.Value)})
		case *model.MinLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMinLength, Arg0: uint32(f.Value)})
		case *model.MaxLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMaxLength, Arg0: uint32(f.Value)})
		case *model.TotalDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FTotalDigits, Arg0: uint32(f.Value)})
		case *model.FractionDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FFractionDigits, Arg0: uint32(f.Value)})
		case model.LexicalFacet:
			op, ok := rangeFacetOp(f.Name())
			if !ok {
				continue
			}
			lexical := f.GetLexical()
			normalized := c.normalizeLexical(lexical, st)
			canon, err := c.canonicalizeNormalizedCore(lexical, normalized, st, nil, canonicalizeGeneral)
			if err != nil {
				return runtime.FacetProgramRef{}, fmt.Errorf("%s: %w", f.Name(), err)
			}
			ref := c.values.addWithHash(canon, runtime.HashBytes(canon))
			c.facets = append(c.facets, runtime.FacetInstr{Op: op, Arg0: ref.Off, Arg1: ref.Len})
		default:
			// ignore unknown facets for now
		}
	}
	return runtime.FacetProgramRef{Off: uint32(start), Len: uint32(len(c.facets) - start)}, nil
}

func (c *compiler) compileEnumeration(enum *model.Enumeration, st *model.SimpleType, partial []model.Facet) (runtime.EnumID, error) {
	if enum == nil {
		return 0, nil
	}
	values := enum.Values()
	if len(values) == 0 {
		return 0, nil
	}
	contexts := enum.ValueContexts()
	if len(contexts) > 0 && len(contexts) != len(values) {
		return 0, fmt.Errorf("enumeration contexts %d do not match values %d", len(contexts), len(values))
	}
	keys := make([]runtime.ValueKey, 0, len(values))
	for i, val := range values {
		var ctx map[string]string
		if len(contexts) > 0 {
			ctx = contexts[i]
		}
		normalized := c.normalizeLexical(val, st)
		if err := c.validatePartialFacets(normalized, st, partial); err != nil {
			return 0, err
		}
		enumKeys, err := c.valueKeysForNormalized(val, normalized, st, ctx)
		if err != nil {
			return 0, err
		}
		keys = append(keys, enumKeys...)
	}
	return c.enums.add(keys), nil
}

func rangeFacetOp(name string) (runtime.FacetOp, bool) {
	switch name {
	case "minInclusive":
		return runtime.FMinInclusive, true
	case "maxInclusive":
		return runtime.FMaxInclusive, true
	case "minExclusive":
		return runtime.FMinExclusive, true
	case "maxExclusive":
		return runtime.FMaxExclusive, true
	default:
		return 0, false
	}
}
