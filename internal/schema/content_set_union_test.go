package schema

import (
	"slices"
	"testing"
)

func TestDFASetUnionReturnedSetsDoNotShareSpareCapacity(t *testing.T) {
	t.Parallel()
	for name, union := range map[string]func(*dfaSetUnion[int], []int) []int{
		"reset":     (*dfaSetUnion[int]).reset,
		"first add": (*dfaSetUnion[int]).add,
		"copying add": func(u *dfaSetUnion[int], set []int) []int {
			u.add([]int{0})
			return u.add(set)
		},
		"owned add": func(u *dfaSetUnion[int], set []int) []int {
			u.add([]int{-2})
			u.add([]int{-1})
			u.add([]int{0})
			return u.add(set)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			backing := make([]int, 3, 16)
			backing[1], backing[2] = 1, 2
			set := backing[1:3]
			var u dfaSetUnion[int]
			got := union(&u, set)
			if len(got) != cap(got) {
				t.Fatalf("returned set %v has capacity %d, want %d", got, cap(got), len(got))
			}
			_ = append(got, 99)
			if slices.Contains(backing[:cap(backing)], 99) {
				t.Fatalf("appending to returned set %v wrote into the storage of %v", got, set)
			}
		})
	}
}
