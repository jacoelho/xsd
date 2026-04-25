package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) buildMatchers(glu *contentmodel.Glushkov) ([]runtime.PosMatcher, error) {
	if glu == nil {
		return nil, fmt.Errorf("runtime build: glushkov model missing")
	}
	matchers := make([]runtime.PosMatcher, len(glu.Positions))
	for i, pos := range glu.Positions {
		switch pos.Kind {
		case contentmodel.PositionElement:
			elemID := runtime.ElemID(pos.ElementID)
			if elemID == 0 || int(elemID) >= len(b.rt.Elements) {
				return nil, fmt.Errorf("runtime build: element %d missing ID", pos.ElementID)
			}
			elem := b.schema.Elements[elemID-1]
			sym, err := b.lookupIRSymbol(elem.Name)
			if err != nil {
				return nil, err
			}
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosExact,
				Sym:  sym,
				Elem: elemID,
			}
		case contentmodel.PositionWildcard:
			rule := runtime.WildcardID(pos.WildcardID)
			if !pos.RuntimeRule {
				var err error
				rule, err = b.addWildcardFromIR(schemair.WildcardID(pos.WildcardID))
				if err != nil {
					return nil, err
				}
			}
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

func (b *schemaBuilder) addWildcardFromIR(id schemair.WildcardID) (runtime.WildcardID, error) {
	if id == 0 || int(id) > len(b.schema.Wildcards) {
		return 0, fmt.Errorf("runtime build: wildcard %d out of range", id)
	}
	if rule, ok := b.wildcardIDs[id]; ok {
		return rule, nil
	}
	rule := b.addWildcardRule(b.schema.Wildcards[id-1])
	b.wildcardIDs[id] = rule
	return rule, nil
}

func (b *schemaBuilder) addWildcardRule(wildcard schemair.Wildcard) runtime.WildcardID {
	rule := runtime.WildcardRule{
		PC:       toRuntimeProcessContents(wildcard.ProcessContents),
		TargetNS: b.internNamespace(wildcard.TargetNamespace),
	}

	off := len(b.wildcardNS)
	switch wildcard.NamespaceKind {
	case schemair.NamespaceOther:
		rule.NS.Kind = runtime.NSOther
		rule.NS.HasTarget = true
	case schemair.NamespaceTarget:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasTarget = true
	case schemair.NamespaceLocal:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasLocal = true
	case schemair.NamespaceNotAbsent:
		rule.NS.Kind = runtime.NSNotAbsent
	case schemair.NamespaceList:
		rule.NS.Kind = runtime.NSEnumeration
		for _, ns := range wildcard.Namespaces {
			if ns == "##targetNamespace" {
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

func toRuntimeProcessContents(process schemair.ProcessContents) runtime.ProcessContents {
	switch process {
	case schemair.ProcessLax:
		return runtime.PCLax
	case schemair.ProcessSkip:
		return runtime.PCSkip
	default:
		return runtime.PCStrict
	}
}
