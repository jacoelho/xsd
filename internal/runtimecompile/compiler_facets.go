package runtimecompile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileFacetProgram(st *types.SimpleType, facets, partial []types.Facet) (runtime.FacetProgramRef, error) {
	if len(facets) == 0 {
		return runtime.FacetProgramRef{}, nil
	}
	start := len(c.facets)
	for _, facet := range facets {
		switch f := facet.(type) {
		case *types.Pattern:
			pid, err := c.addPattern(f)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(pid)})
		case *types.PatternSet:
			pid, err := c.addPatternSet(f)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FPattern, Arg0: uint32(pid)})
		case *types.Enumeration:
			enumID, err := c.compileEnumeration(f, st, partial)
			if err != nil {
				return runtime.FacetProgramRef{}, err
			}
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FEnum, Arg0: uint32(enumID)})
		case *types.Length:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FLength, Arg0: uint32(f.Value)})
		case *types.MinLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMinLength, Arg0: uint32(f.Value)})
		case *types.MaxLength:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMaxLength, Arg0: uint32(f.Value)})
		case *types.TotalDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FTotalDigits, Arg0: uint32(f.Value)})
		case *types.FractionDigits:
			c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FFractionDigits, Arg0: uint32(f.Value)})
		case *types.RangeFacet:
			op, ok := rangeFacetOp(f.Name())
			if !ok {
				return runtime.FacetProgramRef{}, fmt.Errorf("unknown range facet %s", f.Name())
			}
			lexical := f.GetLexical()
			normalized := c.normalizeLexical(lexical, st)
			canon, err := c.canonicalizeNormalized(lexical, normalized, st, nil)
			if err != nil {
				return runtime.FacetProgramRef{}, fmt.Errorf("%s: %w", f.Name(), err)
			}
			stored := canon
			ref := c.values.add(stored)
			c.facets = append(c.facets, runtime.FacetInstr{Op: op, Arg0: ref.Off, Arg1: ref.Len})
		default:
			// ignore unknown facets for now
		}
	}
	return runtime.FacetProgramRef{Off: uint32(start), Len: uint32(len(c.facets) - start)}, nil
}

func (c *compiler) compileEnumeration(enum *types.Enumeration, st *types.SimpleType, partial []types.Facet) (runtime.EnumID, error) {
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

type canonicalizeMode uint8

const (
	// canonicalizeGeneral is used for ordinary validation, applying full facet checks.
	canonicalizeGeneral canonicalizeMode = iota
	// canonicalizeDefault is used for default/fixed values so unions follow runtime default validation order.
	canonicalizeDefault
)

func (c *compiler) validatePartialFacets(normalized string, typ types.Type, facets []types.Facet) error {
	if len(facets) == 0 {
		return nil
	}
	for _, facet := range facets {
		if c.shouldSkipLengthFacet(typ, facet) {
			continue
		}
		switch f := facet.(type) {
		case *types.RangeFacet:
			if err := c.validateRangeFacet(normalized, typ, f); err != nil {
				return err
			}
		case *types.Enumeration:
			// enumeration handled separately
			continue
		case types.LexicalValidator:
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
		default:
			// ignore unsupported facets
		}
	}
	return nil
}

func (c *compiler) validateMemberFacets(normalized string, typ types.Type, facets []types.Facet, ctx map[string]string, includeEnum bool) error {
	if len(facets) == 0 {
		return nil
	}
	for _, facet := range facets {
		if c.shouldSkipLengthFacet(typ, facet) {
			continue
		}
		switch f := facet.(type) {
		case *types.RangeFacet:
			if err := c.validateRangeFacet(normalized, typ, f); err != nil {
				return err
			}
		case *types.Enumeration:
			if !includeEnum {
				continue
			}
			if c.res.isQNameOrNotation(typ) {
				if err := f.ValidateLexicalQName(normalized, typ, ctx); err != nil {
					return err
				}
				continue
			}
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
		case types.LexicalValidator:
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
		default:
			// ignore unsupported facets
		}
	}
	return nil
}

func (c *compiler) validateRangeFacet(normalized string, typ types.Type, facet *types.RangeFacet) error {
	actual, err := c.comparableValue(normalized, typ)
	if err != nil {
		return err
	}
	bound, err := c.comparableValue(facet.GetLexical(), typ)
	if err != nil {
		return err
	}
	cmp, err := actual.Compare(bound)
	if err != nil {
		return err
	}
	ok := false
	switch facet.Name() {
	case "minInclusive":
		ok = cmp >= 0
	case "maxInclusive":
		ok = cmp <= 0
	case "minExclusive":
		ok = cmp > 0
	case "maxExclusive":
		ok = cmp < 0
	default:
		return fmt.Errorf("unknown range facet %s", facet.Name())
	}
	if !ok {
		return fmt.Errorf("facet %s violation", facet.Name())
	}
	return nil
}

func (c *compiler) addPattern(p *types.Pattern) (runtime.PatternID, error) {
	if p.GoPattern == "" {
		if err := p.ValidateSyntax(); err != nil {
			return 0, err
		}
	}
	re, err := regexp.Compile(p.GoPattern)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(p.GoPattern), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func (c *compiler) addPatternSet(set *types.PatternSet) (runtime.PatternID, error) {
	if set == nil || len(set.Patterns) == 0 {
		return 0, nil
	}
	if len(set.Patterns) == 1 {
		return c.addPattern(set.Patterns[0])
	}
	bodies := make([]string, 0, len(set.Patterns))
	for _, pat := range set.Patterns {
		if pat.GoPattern == "" {
			if err := pat.ValidateSyntax(); err != nil {
				return 0, err
			}
		}
		body := stripAnchors(pat.GoPattern)
		bodies = append(bodies, body)
	}
	goPattern := "^(?:" + strings.Join(bodies, "|") + ")$"
	re, err := regexp.Compile(goPattern)
	if err != nil {
		return 0, err
	}
	c.patterns = append(c.patterns, runtime.Pattern{Source: []byte(goPattern), Re: re})
	return runtime.PatternID(len(c.patterns) - 1), nil
}

func (c *compiler) collectFacets(st *types.SimpleType) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if cached, ok := c.facetsCache[st]; ok {
		return cached, nil
	}

	seen := make(map[*types.SimpleType]bool)
	facets, err := c.collectFacetsRecursive(st, seen)
	if err != nil {
		return nil, err
	}
	c.facetsCache[st] = facets
	return facets, nil
}

func (c *compiler) collectFacetsRecursive(st *types.SimpleType, seen map[*types.SimpleType]bool) ([]types.Facet, error) {
	if st == nil {
		return nil, nil
	}
	if seen[st] {
		return nil, nil
	}
	seen[st] = true
	defer delete(seen, st)

	var result []types.Facet
	if base := c.res.baseType(st); base != nil {
		if baseST, ok := types.AsSimpleType(base); ok {
			baseFacets, err := c.collectFacetsRecursive(baseST, seen)
			if err != nil {
				return nil, err
			}
			result = append(result, baseFacets...)
		}
	}

	if st.IsBuiltin() && isBuiltinListName(st.Name().Local) {
		result = append(result, &types.MinLength{Value: 1})
	} else if base := c.res.baseType(st); base != nil {
		if bt := builtinForType(base); bt != nil && isBuiltinListName(bt.Name().Local) {
			result = append(result, &types.MinLength{Value: 1})
		}
	}

	if st.Restriction != nil {
		var stepPatterns []*types.Pattern
		for _, f := range st.Restriction.Facets {
			switch facet := f.(type) {
			case types.Facet:
				if patternFacet, ok := facet.(*types.Pattern); ok {
					if err := patternFacet.ValidateSyntax(); err != nil {
						continue
					}
					stepPatterns = append(stepPatterns, patternFacet)
					continue
				}
				if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
					if err := compilable.ValidateSyntax(); err != nil {
						continue
					}
				}
				result = append(result, facet)
			case *types.DeferredFacet:
				base := c.res.baseType(st)
				resolved, err := typeops.DefaultDeferredFacetConverter(facet, base)
				if err != nil {
					return nil, err
				}
				if resolved != nil {
					result = append(result, resolved)
				}
			}
		}
		if len(stepPatterns) == 1 {
			result = append(result, stepPatterns[0])
		} else if len(stepPatterns) > 1 {
			result = append(result, &types.PatternSet{Patterns: stepPatterns})
		}
	}

	return result, nil
}

func (c *compiler) facetsForType(typ types.Type) ([]types.Facet, error) {
	if st, ok := types.AsSimpleType(typ); ok {
		return c.collectFacets(st)
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		if isBuiltinListName(bt.Name().Local) {
			return []types.Facet{&types.MinLength{Value: 1}}, nil
		}
	}
	return nil, nil
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

func stripAnchors(goPattern string) string {
	const prefix = "^(?:"
	const suffix = ")$"
	if strings.HasPrefix(goPattern, prefix) && strings.HasSuffix(goPattern, suffix) {
		return goPattern[len(prefix) : len(goPattern)-len(suffix)]
	}
	return goPattern
}

func filterFacets(facets []types.Facet, keep func(types.Facet) bool) []types.Facet {
	if len(facets) == 0 {
		return nil
	}
	out := make([]types.Facet, 0, len(facets))
	for _, facet := range facets {
		if keep(facet) {
			out = append(out, facet)
		}
	}
	return out
}

type mapResolver map[string]string

func (c *compiler) shouldSkipLengthFacet(typ types.Type, facet types.Facet) bool {
	if !types.IsLengthFacet(facet) {
		return false
	}
	if c.res.isListType(typ) {
		return false
	}
	return c.res.isQNameOrNotation(typ)
}
