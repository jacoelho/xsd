package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

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

func (c *compiler) shouldSkipLengthFacet(typ types.Type, facet types.Facet) bool {
	if !types.IsLengthFacet(facet) {
		return false
	}
	if c.res.isListType(typ) {
		return false
	}
	return c.res.isQNameOrNotation(typ)
}
