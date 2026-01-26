package types

import (
	"errors"
	"math/big"
	"testing"
)

func TestOccursFromBig(t *testing.T) {
	if _, err := OccursFromBig(nil); err == nil {
		t.Fatalf("expected nil occurs error")
	}
	if _, err := OccursFromBig(big.NewInt(-1)); err == nil {
		t.Fatalf("expected negative occurs error")
	}

	occSmall, err := OccursFromBig(big.NewInt(5))
	if err != nil {
		t.Fatalf("OccursFromBig small error: %v", err)
	}
	if value, ok := occSmall.Int(); !ok || value != 5 {
		t.Fatalf("expected small occurs to fit int")
	}

	maxValue := new(big.Int).SetUint64(uint64(^uint32(0)))
	occMax, err := OccursFromBig(maxValue)
	if err != nil {
		t.Fatalf("OccursFromBig max error: %v", err)
	}
	if value, ok := occMax.Int(); !ok || value <= 0 {
		t.Fatalf("expected max occurs to fit in int on 64-bit")
	}

	tooLarge := new(big.Int).Add(maxValue, big.NewInt(1))
	if _, err := OccursFromBig(tooLarge); err == nil {
		t.Fatalf("expected overflow error for occurs > uint32")
	} else if !errors.Is(err, ErrOccursOverflow) {
		t.Fatalf("expected %v, got %v", ErrOccursOverflow, err)
	}
}

func TestOccursAddMulBig(t *testing.T) {
	maxValue := new(big.Int).SetUint64(uint64(^uint32(0)))
	occMax, err := OccursFromBig(maxValue)
	if err != nil {
		t.Fatalf("OccursFromBig max error: %v", err)
	}

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
	maxValue := new(big.Int).SetUint64(uint64(^uint32(0)))
	occMax, err := OccursFromBig(maxValue)
	if err != nil {
		t.Fatalf("OccursFromBig error: %v", err)
	}

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
	maxValue := new(big.Int).SetUint64(uint64(^uint32(0)))
	occMax, err := OccursFromBig(maxValue)
	if err != nil {
		t.Fatalf("OccursFromBig error: %v", err)
	}
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

	occMax2, err := OccursFromBig(maxValue)
	if err != nil {
		t.Fatalf("OccursFromBig error: %v", err)
	}
	if occMax.Cmp(occMax2) != 0 {
		t.Fatalf("expected max values to compare equal")
	}
}
