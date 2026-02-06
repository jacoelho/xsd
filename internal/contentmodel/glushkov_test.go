package contentmodel

import (
	"math/bits"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func TestGlushkovSequence(t *testing.T) {
	a := elem("a", 1, 1)
	b := elem("b", 1, 1)
	group := sequence(a, b)

	model, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if model.Nullable {
		t.Fatalf("expected non-nullable model")
	}
	if len(model.Positions) != 2 {
		t.Fatalf("positions = %d, want 2", len(model.Positions))
	}
	if model.Positions[0].Element != a || model.Positions[1].Element != b {
		t.Fatalf("position order mismatch")
	}
	if got := bitsetPositions(model.Bitsets, model.First); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("firstPos = %v, want [0]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Last); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("lastPos = %v, want [1]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Follow[0]); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("follow[0] = %v, want [1]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Follow[1]); len(got) != 0 {
		t.Fatalf("follow[1] = %v, want empty", got)
	}
}

func TestGlushkovChoice(t *testing.T) {
	a := elem("a", 1, 1)
	b := elem("b", 1, 1)
	group := choice(1, 1, a, b)

	model, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if got := bitsetPositions(model.Bitsets, model.First); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("firstPos = %v, want [0 1]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Last); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("lastPos = %v, want [0 1]", got)
	}
	for i := range model.Follow {
		if got := bitsetPositions(model.Bitsets, model.Follow[i]); len(got) != 0 {
			t.Fatalf("follow[%d] = %v, want empty", i, got)
		}
	}
}

func TestGlushkovNullableSequence(t *testing.T) {
	a := elem("a", 0, 1)
	b := elem("b", 1, 1)
	group := sequence(a, b)

	model, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if model.Nullable {
		t.Fatalf("expected non-nullable model")
	}
	if got := bitsetPositions(model.Bitsets, model.First); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("firstPos = %v, want [0 1]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Last); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("lastPos = %v, want [1]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Follow[0]); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("follow[0] = %v, want [1]", got)
	}
}

func TestGlushkovStar(t *testing.T) {
	a := elem("a", 0, -1)
	model, err := BuildGlushkov(a)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if !model.Nullable {
		t.Fatalf("expected nullable model")
	}
	if got := bitsetPositions(model.Bitsets, model.First); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("firstPos = %v, want [0]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Last); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("lastPos = %v, want [0]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Follow[0]); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("follow[0] = %v, want [0]", got)
	}
}

func TestGlushkovBoundedOccurs(t *testing.T) {
	a := elem("a", 2, 3)
	model, err := BuildGlushkov(a)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if model.Nullable {
		t.Fatalf("expected non-nullable model")
	}
	if len(model.Positions) != 3 {
		t.Fatalf("positions = %d, want 3", len(model.Positions))
	}
	if got := bitsetPositions(model.Bitsets, model.First); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("firstPos = %v, want [0]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Last); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("lastPos = %v, want [1 2]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Follow[0]); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("follow[0] = %v, want [1]", got)
	}
	if got := bitsetPositions(model.Bitsets, model.Follow[1]); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("follow[1] = %v, want [2]", got)
	}
}

func bitsetPositions(blob runtime.BitsetBlob, ref runtime.BitsetRef) []int {
	if ref.Len == 0 {
		return nil
	}
	words := blob.Words[ref.Off : ref.Off+ref.Len]
	out := make([]int, 0, len(words))
	for i, w := range words {
		for w != 0 {
			bit := bits.TrailingZeros64(w)
			out = append(out, i*64+bit)
			w &^= 1 << bit
		}
	}
	return out
}

func elem(local string, minOccurs, maxOccurs int) *types.ElementDecl {
	decl := &types.ElementDecl{
		Name: types.QName{Local: local},
	}
	decl.MinOccurs = types.OccursFromInt(minOccurs)
	if maxOccurs < 0 {
		decl.MaxOccurs = types.OccursUnbounded
	} else {
		decl.MaxOccurs = types.OccursFromInt(maxOccurs)
	}
	return decl
}

func sequence(particles ...types.Particle) *types.ModelGroup {
	group := &types.ModelGroup{
		Kind:      types.Sequence,
		Particles: particles,
	}
	group.MinOccurs = types.OccursFromInt(1)
	group.MaxOccurs = types.OccursFromInt(1)
	return group
}

func choice(minOccurs, maxOccurs int, particles ...types.Particle) *types.ModelGroup {
	group := &types.ModelGroup{
		Kind:      types.Choice,
		Particles: particles,
	}
	group.MinOccurs = types.OccursFromInt(minOccurs)
	if maxOccurs < 0 {
		group.MaxOccurs = types.OccursUnbounded
	} else {
		group.MaxOccurs = types.OccursFromInt(maxOccurs)
	}
	return group
}
