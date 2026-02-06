package contentmodel

import "fmt"

// PositionOverlapFunc reports whether two positions can match the same element.
type PositionOverlapFunc func(left, right Position) bool

// CheckDeterminism reports a UPA violation if a reachable state contains overlapping positions.
func CheckDeterminism(glu *Glushkov, overlap PositionOverlapFunc) error {
	if glu == nil || overlap == nil {
		return nil
	}
	if len(glu.Positions) == 0 {
		return nil
	}
	if glu.firstRaw == nil || len(glu.followRaw) == 0 {
		return fmt.Errorf("glushkov raw bitsets missing")
	}

	overlaps := buildOverlapSets(glu.Positions, overlap)
	if overlaps == nil {
		return nil
	}
	if err := checkSetDeterminism(glu.firstRaw, overlaps); err != nil {
		return err
	}
	for _, state := range glu.followRaw {
		if state == nil || state.empty() {
			continue
		}
		if err := checkSetDeterminism(state, overlaps); err != nil {
			return err
		}
	}
	return nil
}

func buildOverlapSets(positions []Position, overlap PositionOverlapFunc) []*bitset {
	if len(positions) == 0 || overlap == nil {
		return nil
	}
	var overlaps []*bitset
	for i := range positions {
		for j := i + 1; j < len(positions); j++ {
			if !overlap(positions[i], positions[j]) {
				continue
			}
			if overlaps == nil {
				overlaps = make([]*bitset, len(positions))
			}
			if overlaps[i] == nil {
				overlaps[i] = newBitset(len(positions))
			}
			if overlaps[j] == nil {
				overlaps[j] = newBitset(len(positions))
			}
			overlaps[i].set(j)
			overlaps[j].set(i)
		}
	}
	return overlaps
}

func checkSetDeterminism(state *bitset, overlaps []*bitset) error {
	if state == nil || state.empty() || overlaps == nil {
		return nil
	}
	var err error
	state.forEach(func(pos int) {
		if err != nil {
			return
		}
		if pos < 0 || pos >= len(overlaps) {
			return
		}
		overlapSet := overlaps[pos]
		if overlapSet == nil {
			return
		}
		if other, ok := state.intersectionIndex(overlapSet); ok {
			err = fmt.Errorf("content model is not deterministic: positions %d and %d overlap", pos, other)
		}
	})
	return err
}
