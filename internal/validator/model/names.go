package model

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

const maxExpectedElements = 32

func normalizeExpectedElements(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	filtered := names[:0]
	for _, name := range names {
		if name == "" {
			continue
		}
		filtered = append(filtered, name)
	}
	if len(filtered) == 0 {
		return nil
	}
	slices.Sort(filtered)
	filtered = slices.Compact(filtered)
	if len(filtered) > maxExpectedElements {
		filtered = filtered[:maxExpectedElements]
	}
	return slices.Clone(filtered)
}

// ActualElementName formats one actual element name for validation details.
func ActualElementName(rt *runtime.Schema, sym runtime.SymbolID, nsID runtime.NamespaceID) string {
	if sym != 0 {
		return symbolName(rt, sym)
	}
	if nsID != 0 {
		return "{?}?"
	}
	return ""
}

// ExpectedGlobalElements reports the set of declared root element names.
func ExpectedGlobalElements(rt *runtime.Schema) []string {
	if rt == nil {
		return nil
	}
	names := make([]string, 0, len(rt.GlobalElements))
	for sym, elem := range rt.GlobalElements {
		if sym == 0 || elem == 0 {
			continue
		}
		name := elementName(rt, elem)
		if name == "" {
			name = symbolName(rt, runtime.SymbolID(sym))
		}
		names = append(names, name)
	}
	return normalizeExpectedElements(names)
}

func expectedAllMemberNames(rt *runtime.Schema, member runtime.AllMember) []string {
	names := make([]string, 0, 4)
	if head := elementName(rt, member.Elem); head != "" {
		names = append(names, head)
	}
	if rt == nil || !member.AllowsSubst || member.SubstLen == 0 {
		return names
	}
	start := int(member.SubstOff)
	end := start + int(member.SubstLen)
	if start < 0 || end < 0 || end > len(rt.Models.AllSubst) {
		return names
	}
	for _, elem := range rt.Models.AllSubst[start:end] {
		names = append(names, elementName(rt, elem))
	}
	return names
}

func expectedFromAllRemaining(rt *runtime.Schema, model *runtime.AllModel, state []uint64, onlyRequired bool) []string {
	if model == nil {
		return nil
	}
	names := make([]string, 0, len(model.Members))
	for i, member := range model.Members {
		if allHas(state, i) {
			continue
		}
		if onlyRequired && member.Optional {
			continue
		}
		names = append(names, expectedAllMemberNames(rt, member)...)
	}
	return normalizeExpectedElements(names)
}

func expectedFromDFAState(rt *runtime.Schema, model *runtime.DFAModel, state uint32) []string {
	if model == nil || int(state) >= len(model.States) {
		return nil
	}
	rec := model.States[state]
	trans, err := sliceDFATransitions(model, rec)
	if err != nil {
		return nil
	}
	wild, err := sliceDFAWildcards(model, rec)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(trans)+len(wild))
	for _, tr := range trans {
		name := elementName(rt, tr.Elem)
		if name == "" {
			name = symbolName(rt, tr.Sym)
		}
		names = append(names, name)
	}
	for range wild {
		names = append(names, "*")
	}
	return normalizeExpectedElements(names)
}

func expectedFromNFAMatchers(rt *runtime.Schema, model *runtime.NFAModel, positions []uint64) []string {
	if model == nil || len(positions) == 0 {
		return nil
	}
	names := make([]string, 0, 4)
	forEachBit(positions, len(model.Matchers), func(pos int) {
		m := model.Matchers[pos]
		switch m.Kind {
		case runtime.PosExact:
			name := elementName(rt, m.Elem)
			if name == "" {
				name = symbolName(rt, m.Sym)
			}
			names = append(names, name)
		case runtime.PosWildcard:
			names = append(names, "*")
		}
	})
	return normalizeExpectedElements(names)
}

func expectedFromNFAStart(rt *runtime.Schema, model *runtime.NFAModel) []string {
	if model == nil {
		return nil
	}
	start, ok := bitsetSlice(model.Bitsets, model.Start)
	if !ok {
		return nil
	}
	return expectedFromNFAMatchers(rt, model, start)
}

func expectedFromNFAFollow(rt *runtime.Schema, model *runtime.NFAModel, state []uint64) []string {
	if model == nil || len(state) == 0 || int(model.FollowLen) > len(model.Follow) {
		return nil
	}
	follow := make([]uint64, len(state))
	forEachBit(state, len(model.Follow), func(pos int) {
		ref := model.Follow[pos]
		set, ok := bitsetSlice(model.Bitsets, ref)
		if !ok {
			return
		}
		bitsetOr(follow, set)
	})
	return expectedFromNFAMatchers(rt, model, follow)
}

func symbolName(rt *runtime.Schema, sym runtime.SymbolID) string {
	if rt == nil || sym == 0 {
		return ""
	}
	local := rt.Symbols.LocalBytes(sym)
	if len(local) == 0 {
		return ""
	}
	nsID := rt.Symbols.NS[sym]
	ns := rt.Namespaces.Bytes(nsID)
	if len(ns) == 0 {
		return string(local)
	}
	return "{" + string(ns) + "}" + string(local)
}

func elementName(rt *runtime.Schema, elem runtime.ElemID) string {
	if rt == nil || elem == 0 || int(elem) >= len(rt.Elements) {
		return ""
	}
	return symbolName(rt, rt.Elements[elem].Name)
}
