package types

import "testing"

func TestOccursFromUint64(t *testing.T) {
	occSmall := OccursFromUint64(5)
	if value, ok := occSmall.Int(); !ok || value != 5 {
		t.Fatalf("expected small occurs to fit int")
	}
	if occSmall.IsOverflow() {
		t.Fatalf("expected small occurs not to overflow")
	}

	maxValue := uint64(^uint32(0))
	occMax := OccursFromUint64(maxValue)
	if occMax.IsOverflow() {
		t.Fatalf("expected max occurs to fit uint32")
	}
	if occMax.CmpInt(1) <= 0 {
		t.Fatalf("expected max occurs to exceed 1")
	}

	occTooLarge := OccursFromUint64(maxValue + 1)
	if !occTooLarge.IsOverflow() {
		t.Fatalf("expected occurs to overflow when exceeding uint32")
	}
}

func TestOccursAddMulBig(t *testing.T) {
	maxValue := uint64(^uint32(0))
	occMax := OccursFromUint64(maxValue)

	sum := AddOccurs(occMax, OccursFromInt(1))
	if !sum.IsOverflow() {
		t.Fatalf("expected sum overflow")
	}

	product := MulOccurs(occMax, OccursFromInt(2))
	if !product.IsOverflow() {
		t.Fatalf("expected product overflow")
	}

	if got := MulOccurs(OccursFromInt(0), OccursUnbounded); !got.IsZero() {
		t.Fatalf("expected zero product")
	}
}

func TestOccursMinMaxWithBig(t *testing.T) {
	maxValue := uint64(^uint32(0))
	occMax := OccursFromUint64(maxValue)

	if got := MinOccurs(OccursUnbounded, occMax); !got.Equal(occMax) {
		t.Fatalf("expected min to pick bounded value")
	}
	if got := MaxOccurs(OccursUnbounded, occMax); !got.IsUnbounded() {
		t.Fatalf("expected max to pick unbounded")
	}

	small := OccursFromInt(1)
	if got := MinOccurs(small, occMax); !got.Equal(small) {
		t.Fatalf("expected min to pick small")
	}
	if got := MaxOccurs(small, occMax); !got.Equal(occMax) {
		t.Fatalf("expected max to pick max")
	}
}

func TestOccursCmpBig(t *testing.T) {
	maxValue := uint64(^uint32(0))
	occMax := OccursFromUint64(maxValue)
	occSmall := OccursFromInt(1)

	if occMax.Cmp(occSmall) <= 0 {
		t.Fatalf("expected max > small")
	}
	if occSmall.Cmp(occMax) >= 0 {
		t.Fatalf("expected small < max")
	}
	if occMax.Cmp(OccursUnbounded) >= 0 {
		t.Fatalf("expected max < unbounded")
	}
	if OccursUnbounded.Cmp(occMax) <= 0 {
		t.Fatalf("expected unbounded > max")
	}
	if occMax.CmpInt(1) <= 0 {
		t.Fatalf("expected max to exceed small")
	}

	occMax2 := OccursFromUint64(maxValue)
	if occMax.Cmp(occMax2) != 0 {
		t.Fatalf("expected max values to compare equal")
	}
}
