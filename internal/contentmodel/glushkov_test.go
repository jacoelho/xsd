package contentmodel

import (
	"math/bits"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestGlushkovSequence(t *testing.T) {
	a := elem("a", 1, 1)
	b := elem("b", 1, 1)
	group := sequence(a, b)

	gluModel, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if gluModel.Nullable {
		t.Fatalf("expected non-nullable model")
	}
	if len(gluModel.Positions) != 2 {
		t.Fatalf("positions = %d, want 2", len(gluModel.Positions))
	}
	if gluModel.Positions[0].Element != a || gluModel.Positions[1].Element != b {
		t.Fatalf("position order mismatch")
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.First); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("firstPos = %v, want [0]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Last); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("lastPos = %v, want [1]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[0]); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("follow[0] = %v, want [1]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[1]); len(got) != 0 {
		t.Fatalf("follow[1] = %v, want empty", got)
	}
}

func TestGlushkovChoice(t *testing.T) {
	a := elem("a", 1, 1)
	b := elem("b", 1, 1)
	group := choice(1, 1, a, b)

	gluModel, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.First); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("firstPos = %v, want [0 1]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Last); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("lastPos = %v, want [0 1]", got)
	}
	for i := range gluModel.Follow {
		if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[i]); len(got) != 0 {
			t.Fatalf("follow[%d] = %v, want empty", i, got)
		}
	}
}

func TestGlushkovNullableSequence(t *testing.T) {
	a := elem("a", 0, 1)
	b := elem("b", 1, 1)
	group := sequence(a, b)

	gluModel, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if gluModel.Nullable {
		t.Fatalf("expected non-nullable model")
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.First); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("firstPos = %v, want [0 1]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Last); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("lastPos = %v, want [1]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[0]); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("follow[0] = %v, want [1]", got)
	}
}

func TestGlushkovStar(t *testing.T) {
	a := elem("a", 0, -1)
	gluModel, err := BuildGlushkov(a)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if !gluModel.Nullable {
		t.Fatalf("expected nullable model")
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.First); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("firstPos = %v, want [0]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Last); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("lastPos = %v, want [0]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[0]); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("follow[0] = %v, want [0]", got)
	}
}

func TestGlushkovBoundedOccurs(t *testing.T) {
	a := elem("a", 2, 3)
	gluModel, err := BuildGlushkov(a)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	if gluModel.Nullable {
		t.Fatalf("expected non-nullable model")
	}
	if len(gluModel.Positions) != 3 {
		t.Fatalf("positions = %d, want 3", len(gluModel.Positions))
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.First); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("firstPos = %v, want [0]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Last); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("lastPos = %v, want [1 2]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[0]); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("follow[0] = %v, want [1]", got)
	}
	if got := bitsetPositions(gluModel.Bitsets, gluModel.Follow[1]); !reflect.DeepEqual(got, []int{2}) {
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

func elem(local string, minOccurs, maxOccurs int) *model.ElementDecl {
	decl := &model.ElementDecl{
		Name: model.QName{Local: local},
	}
	decl.MinOccurs = model.OccursFromInt(minOccurs)
	if maxOccurs < 0 {
		decl.MaxOccurs = model.OccursUnbounded
	} else {
		decl.MaxOccurs = model.OccursFromInt(maxOccurs)
	}
	return decl
}

func sequence(particles ...model.Particle) *model.ModelGroup {
	group := &model.ModelGroup{
		Kind:      model.Sequence,
		Particles: particles,
	}
	group.MinOccurs = model.OccursFromInt(1)
	group.MaxOccurs = model.OccursFromInt(1)
	return group
}

func choice(minOccurs, maxOccurs int, particles ...model.Particle) *model.ModelGroup {
	group := &model.ModelGroup{
		Kind:      model.Choice,
		Particles: particles,
	}
	group.MinOccurs = model.OccursFromInt(minOccurs)
	if maxOccurs < 0 {
		group.MaxOccurs = model.OccursUnbounded
	} else {
		group.MaxOccurs = model.OccursFromInt(maxOccurs)
	}
	return group
}
