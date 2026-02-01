package models

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

// ExpandSubstitution expands element positions to include substitution group members.
// resolve maps reference elements to their substitution group head declarations when needed.
// members returns the substitution closure members for a head element.
func ExpandSubstitution(glu *Glushkov, resolve func(*types.ElementDecl) *types.ElementDecl, members func(*types.ElementDecl) []*types.ElementDecl) (*Glushkov, error) {
	if glu == nil {
		return nil, fmt.Errorf("glushkov model is nil")
	}
	if members == nil || len(glu.Positions) == 0 {
		return glu, nil
	}

	mapping := make([][]int, len(glu.Positions))
	positions := make([]Position, 0, len(glu.Positions))
	expanded := false

	for i, pos := range glu.Positions {
		switch pos.Kind {
		case PositionWildcard:
			mapping[i] = []int{len(positions)}
			positions = append(positions, pos)
		case PositionElement:
			if pos.Element == nil {
				return nil, fmt.Errorf("position %d has nil element", i)
			}
			if !pos.AllowsSubst {
				mapping[i] = []int{len(positions)}
				positions = append(positions, pos)
				continue
			}
			head := pos.Element
			if resolve != nil {
				if resolved := resolve(pos.Element); resolved != nil {
					head = resolved
				}
			}
			list, err := ExpandSubstitutionMembers(head, members)
			if err != nil {
				return nil, err
			}
			if len(list) != 1 || list[0] != pos.Element {
				expanded = true
			}
			for _, member := range list {
				idx := len(positions)
				mapping[i] = append(mapping[i], idx)
				positions = append(positions, Position{
					Kind:        PositionElement,
					Element:     member,
					AllowsSubst: pos.AllowsSubst,
				})
			}
		default:
			return nil, fmt.Errorf("position %d has unsupported kind %d", i, pos.Kind)
		}
	}

	if !expanded {
		return glu, nil
	}

	newSize := len(positions)
	if newSize == 0 {
		return &Glushkov{Nullable: glu.Nullable}, nil
	}

	expand := func(set *bitset) *bitset {
		out := newBitset(newSize)
		if set == nil {
			return out
		}
		set.forEach(func(pos int) {
			if pos >= len(mapping) {
				return
			}
			for _, mapped := range mapping[pos] {
				out.set(mapped)
			}
		})
		return out
	}

	first := expand(glu.firstRaw)
	last := expand(glu.lastRaw)
	follow := make([]*bitset, newSize)
	cache := make([]*bitset, len(glu.followRaw))
	for i, set := range glu.followRaw {
		cache[i] = expand(set)
	}
	for i, mapped := range mapping {
		if i >= len(cache) {
			continue
		}
		for _, dst := range mapped {
			follow[dst] = cache[i]
		}
	}

	var blob runtime.BitsetBlob
	firstRef := packBitset(&blob, first)
	lastRef := packBitset(&blob, last)
	followRefs := make([]runtime.BitsetRef, newSize)
	for i, set := range follow {
		followRefs[i] = packBitset(&blob, set)
	}

	return &Glushkov{
		Positions: positions,
		First:     firstRef,
		Last:      lastRef,
		Follow:    followRefs,
		Nullable:  glu.Nullable,
		Bitsets:   blob,
		firstRaw:  first,
		lastRaw:   last,
		followRaw: follow,
	}, nil
}

// ExpandSubstitutionMembers returns the allowed substitution members for a head element.
// It applies blocking, final, and abstract rules to produce the substitutable set.
func ExpandSubstitutionMembers(head *types.ElementDecl, members func(*types.ElementDecl) []*types.ElementDecl) ([]*types.ElementDecl, error) {
	if head == nil {
		return nil, fmt.Errorf("head element is nil")
	}
	if head.Block.Has(types.DerivationSubstitution) {
		if head.Abstract {
			return nil, fmt.Errorf("abstract head %s blocks substitution", head.Name)
		}
		return []*types.ElementDecl{head}, nil
	}

	blocked := blockedDerivations(head)
	seen := make(map[*types.ElementDecl]bool)
	out := make([]*types.ElementDecl, 0)
	if !head.Abstract {
		out = append(out, head)
		seen[head] = true
	}

	memberList := members(head)
	hasMember := false
	for _, member := range memberList {
		if member == nil || seen[member] {
			continue
		}
		hasMember = true
		seen[member] = true
		if member.Abstract {
			continue
		}
		if blocked != 0 && head.Type != nil && member.Type != nil {
			if mask, ok := derivationMask(member.Type, head.Type); ok {
				if mask&blocked != 0 {
					continue
				}
			}
		}
		out = append(out, member)
	}

	if len(out) == 0 {
		if head.Abstract && !hasMember {
			return out, nil
		}
		return nil, fmt.Errorf("abstract head %s has no substitutable members", head.Name)
	}
	return out, nil
}

func blockedDerivations(head *types.ElementDecl) types.DerivationMethod {
	var mask types.DerivationMethod
	if head.Block.Has(types.DerivationExtension) {
		mask |= types.DerivationExtension
	}
	if head.Block.Has(types.DerivationRestriction) {
		mask |= types.DerivationRestriction
	}
	if head.Final.Has(types.DerivationExtension) {
		mask |= types.DerivationExtension
	}
	if head.Final.Has(types.DerivationRestriction) {
		mask |= types.DerivationRestriction
	}
	if head.Final.Has(types.DerivationList) {
		mask |= types.DerivationList
	}
	if head.Final.Has(types.DerivationUnion) {
		mask |= types.DerivationUnion
	}

	switch typ := head.Type.(type) {
	case *types.ComplexType:
		if typ.Block.Has(types.DerivationExtension) {
			mask |= types.DerivationExtension
		}
		if typ.Block.Has(types.DerivationRestriction) {
			mask |= types.DerivationRestriction
		}
		if typ.Final.Has(types.DerivationExtension) {
			mask |= types.DerivationExtension
		}
		if typ.Final.Has(types.DerivationRestriction) {
			mask |= types.DerivationRestriction
		}
	case *types.SimpleType:
		if typ.Final.Has(types.DerivationRestriction) {
			mask |= types.DerivationRestriction
		}
		if typ.Final.Has(types.DerivationList) {
			mask |= types.DerivationList
		}
		if typ.Final.Has(types.DerivationUnion) {
			mask |= types.DerivationUnion
		}
	}

	return mask
}

func derivationMask(derived, base types.Type) (types.DerivationMethod, bool) {
	if derived == nil || base == nil {
		return 0, false
	}
	if derived == base {
		return 0, true
	}
	mask := types.DerivationMethod(0)
	seen := make(map[types.Type]bool)
	current := derived
	for current != nil && current != base {
		if seen[current] {
			break
		}
		seen[current] = true
		next, method := derivationStep(current)
		if next == nil {
			break
		}
		mask |= method
		current = next
	}
	if current == base {
		return mask, true
	}
	return 0, false
}

func derivationStep(current types.Type) (types.Type, types.DerivationMethod) {
	switch typed := current.(type) {
	case *types.ComplexType:
		method := typed.DerivationMethod
		if method == 0 {
			method = types.DerivationRestriction
		}
		if typed.ResolvedBase != nil {
			return typed.ResolvedBase, method
		}
		return typed.BaseType(), method
	case *types.SimpleType:
		switch {
		case typed.List != nil:
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationList
		case typed.Union != nil:
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationUnion
		default:
			if typed.ResolvedBase != nil {
				return typed.ResolvedBase, types.DerivationRestriction
			}
			if typed.Restriction != nil && typed.Restriction.SimpleType != nil {
				return typed.Restriction.SimpleType, types.DerivationRestriction
			}
			return nil, 0
		}
	case *types.BuiltinType:
		base := typed.BaseType()
		if base == nil {
			return nil, 0
		}
		return base, types.DerivationRestriction
	default:
		return nil, 0
	}
}
