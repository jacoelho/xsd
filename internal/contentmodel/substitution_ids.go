package contentmodel

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func ExpandSubstitutionIDs(glu *Glushkov, members func(uint32) ([]uint32, error)) (*Glushkov, error) {
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
			if !pos.AllowsSubst {
				mapping[i] = []int{len(positions)}
				positions = append(positions, pos)
				continue
			}
			list, err := members(pos.ElementID)
			if err != nil {
				return nil, err
			}
			if len(list) == 0 {
				list = []uint32{pos.ElementID}
			}
			if len(list) != 1 || list[0] != pos.ElementID {
				expanded = true
			}
			for _, member := range list {
				idx := len(positions)
				mapping[i] = append(mapping[i], idx)
				positions = append(positions, Position{
					Kind:        PositionElement,
					ElementID:   member,
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
