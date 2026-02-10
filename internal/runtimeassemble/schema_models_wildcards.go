package runtimeassemble

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (b *schemaBuilder) buildMatchers(glu *models.Glushkov) ([]runtime.PosMatcher, error) {
	if glu == nil {
		return nil, fmt.Errorf("runtime build: glushkov model missing")
	}
	matchers := make([]runtime.PosMatcher, len(glu.Positions))
	for i, pos := range glu.Positions {
		switch pos.Kind {
		case models.PositionElement:
			if pos.Element == nil {
				return nil, fmt.Errorf("runtime build: position %d missing element", i)
			}
			elemID, ok := b.runtimeElemID(pos.Element)
			if !ok {
				return nil, fmt.Errorf("runtime build: element %s missing ID", pos.Element.Name)
			}
			sym := b.internQName(pos.Element.Name)
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosExact,
				Sym:  sym,
				Elem: elemID,
			}
		case models.PositionWildcard:
			if pos.Wildcard == nil {
				return nil, fmt.Errorf("runtime build: position %d missing wildcard", i)
			}
			rule := b.addWildcardAnyElement(pos.Wildcard)
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosWildcard,
				Rule: rule,
			}
		default:
			return nil, fmt.Errorf("runtime build: unknown position kind %d", pos.Kind)
		}
	}
	return matchers, nil
}

func (b *schemaBuilder) addWildcardAnyElement(anyElem *model.AnyElement) runtime.WildcardID {
	if anyElem == nil {
		return 0
	}
	if b.anyElementRules == nil {
		b.anyElementRules = make(map[*model.AnyElement]runtime.WildcardID)
	}
	if id, ok := b.anyElementRules[anyElem]; ok {
		return id
	}
	id := b.addWildcard(anyElem.Namespace, anyElem.NamespaceList, anyElem.TargetNamespace, anyElem.ProcessContents)
	b.anyElementRules[anyElem] = id
	return id
}

func (b *schemaBuilder) addWildcard(constraint model.NamespaceConstraint, list []model.NamespaceURI, target model.NamespaceURI, pc model.ProcessContents) runtime.WildcardID {
	rule := runtime.WildcardRule{
		PC:       toRuntimeProcessContents(pc),
		TargetNS: b.internNamespace(target),
	}

	off := len(b.wildcardNS)
	switch constraint {
	case model.NSCAny:
		rule.NS.Kind = runtime.NSAny
	case model.NSCOther:
		rule.NS.Kind = runtime.NSOther
		rule.NS.HasTarget = true
	case model.NSCTargetNamespace:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasTarget = true
	case model.NSCLocal:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasLocal = true
	case model.NSCNotAbsent:
		rule.NS.Kind = runtime.NSNotAbsent
	case model.NSCList:
		rule.NS.Kind = runtime.NSEnumeration
		for _, ns := range list {
			if ns == model.NamespaceTargetPlaceholder {
				rule.NS.HasTarget = true
				continue
			}
			if ns == "" {
				rule.NS.HasLocal = true
				continue
			}
			b.wildcardNS = append(b.wildcardNS, b.internNamespace(ns))
		}
	default:
		rule.NS.Kind = runtime.NSAny
	}

	if rule.NS.Kind == runtime.NSEnumeration {
		ln := len(b.wildcardNS) - off
		if ln > 0 {
			rule.NS.Off = uint32(off)
			rule.NS.Len = uint32(ln)
		}
	}

	b.wildcards = append(b.wildcards, rule)
	return runtime.WildcardID(len(b.wildcards) - 1)
}
