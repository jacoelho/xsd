package runtimecompile

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
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

func (b *schemaBuilder) addWildcardAnyElement(anyElem *types.AnyElement) runtime.WildcardID {
	if anyElem == nil {
		return 0
	}
	if b.anyElementRules == nil {
		b.anyElementRules = make(map[*types.AnyElement]runtime.WildcardID)
	}
	if id, ok := b.anyElementRules[anyElem]; ok {
		return id
	}
	id := b.addWildcard(anyElem.Namespace, anyElem.NamespaceList, anyElem.TargetNamespace, anyElem.ProcessContents)
	b.anyElementRules[anyElem] = id
	return id
}

func (b *schemaBuilder) addWildcardAnyAttribute(anyAttr *types.AnyAttribute) runtime.WildcardID {
	if anyAttr == nil {
		return 0
	}
	return b.addWildcard(anyAttr.Namespace, anyAttr.NamespaceList, anyAttr.TargetNamespace, anyAttr.ProcessContents)
}

func (b *schemaBuilder) addWildcard(constraint types.NamespaceConstraint, list []types.NamespaceURI, target types.NamespaceURI, pc types.ProcessContents) runtime.WildcardID {
	rule := runtime.WildcardRule{
		PC:       toRuntimeProcessContents(pc),
		TargetNS: b.internNamespace(target),
	}

	off := len(b.wildcardNS)
	switch constraint {
	case types.NSCAny:
		rule.NS.Kind = runtime.NSAny
	case types.NSCOther:
		rule.NS.Kind = runtime.NSOther
		rule.NS.HasTarget = true
	case types.NSCTargetNamespace:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasTarget = true
	case types.NSCLocal:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasLocal = true
	case types.NSCNotAbsent:
		rule.NS.Kind = runtime.NSNotAbsent
	case types.NSCList:
		rule.NS.Kind = runtime.NSEnumeration
		for _, ns := range list {
			if ns == types.NamespaceTargetPlaceholder {
				rule.NS.HasTarget = true
				continue
			}
			if ns.IsEmpty() {
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
