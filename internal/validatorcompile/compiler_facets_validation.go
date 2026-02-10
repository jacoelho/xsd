package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/typefacet"
)

type canonicalizeMode uint8

const (
	// canonicalizeGeneral is used for ordinary validation, applying full facet checks.
	canonicalizeGeneral canonicalizeMode = iota
	// canonicalizeDefault is used for default/fixed values so unions follow runtime default validation order.
	canonicalizeDefault
)

func (c *compiler) validatePartialFacets(normalized string, typ model.Type, facets []model.Facet) error {
	if len(facets) == 0 {
		return nil
	}
	for _, facet := range facets {
		if c.shouldSkipLengthFacet(typ, facet) {
			continue
		}
		if facetName, lexical, ok := rangeFacetLexical(facet); ok {
			if err := c.validateRangeFacet(normalized, typ, facetName, lexical); err != nil {
				return err
			}
			continue
		}
		switch f := facet.(type) {
		case *model.Enumeration:
			// enumeration handled separately
			continue
		case model.LexicalValidator:
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
		default:
			// ignore unsupported facets
		}
	}
	return nil
}

func (c *compiler) validateMemberFacets(normalized string, typ model.Type, facets []model.Facet, ctx map[string]string, includeEnum bool) error {
	if len(facets) == 0 {
		return nil
	}
	for _, facet := range facets {
		if c.shouldSkipLengthFacet(typ, facet) {
			continue
		}
		if facetName, lexical, ok := rangeFacetLexical(facet); ok {
			if err := c.validateRangeFacet(normalized, typ, facetName, lexical); err != nil {
				return err
			}
			continue
		}
		switch f := facet.(type) {
		case *model.Enumeration:
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
		case model.LexicalValidator:
			if err := f.ValidateLexical(normalized, typ); err != nil {
				return err
			}
		default:
			// ignore unsupported facets
		}
	}
	return nil
}

func (c *compiler) validateRangeFacet(normalized string, typ model.Type, facetName, facetLexical string) error {
	actual, err := c.comparableValue(normalized, typ)
	if err != nil {
		return err
	}
	bound, err := c.comparableValue(facetLexical, typ)
	if err != nil {
		return err
	}
	cmp, err := actual.Compare(bound)
	if err != nil {
		return err
	}
	ok := false
	switch facetName {
	case "minInclusive":
		ok = cmp >= 0
	case "maxInclusive":
		ok = cmp <= 0
	case "minExclusive":
		ok = cmp > 0
	case "maxExclusive":
		ok = cmp < 0
	default:
		return fmt.Errorf("unknown range facet %s", facetName)
	}
	if !ok {
		return fmt.Errorf("facet %s violation", facetName)
	}
	return nil
}

func rangeFacetLexical(facet model.Facet) (string, string, bool) {
	facetName := facet.Name()
	if _, ok := rangeFacetOp(facetName); !ok {
		return "", "", false
	}
	lexicalFacet, ok := facet.(model.LexicalFacet)
	if !ok {
		return "", "", false
	}
	return facetName, lexicalFacet.GetLexical(), true
}

func (c *compiler) shouldSkipLengthFacet(typ model.Type, facet model.Facet) bool {
	if !typefacet.IsLengthFacet(facet) {
		return false
	}
	if c.res.isListType(typ) {
		return false
	}
	return c.res.isQNameOrNotation(typ)
}
