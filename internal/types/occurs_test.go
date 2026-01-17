package types

import (
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

	maxInt := int(^uint(0) >> 1)
	bigValue := new(big.Int).Add(big.NewInt(int64(maxInt)), big.NewInt(1))
	occBig, err := OccursFromBig(bigValue)
	if err != nil {
		t.Fatalf("OccursFromBig big error: %v", err)
	}
	if _, ok := occBig.Int(); ok {
		t.Fatalf("expected big occurs to not fit in int")
	}
	if occBig.CmpInt(maxInt) <= 0 {
		t.Fatalf("expected big occurs to exceed maxInt")
	}
	if occBig.String() != bigValue.String() {
		t.Fatalf("unexpected big occurs string: %s", occBig)
	}
}

func TestOccursAddMulBig(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	sum := AddOccurs(OccursFromInt(maxInt), OccursFromInt(1))
	if sum.IsUnbounded() {
		t.Fatalf("unexpected unbounded sum")
	}
	if _, ok := sum.Int(); ok {
		t.Fatalf("expected sum to not fit in int")
	}
	if sum.CmpInt(maxInt) <= 0 {
		t.Fatalf("expected sum to exceed maxInt")
	}

	bigA := new(big.Int).Add(big.NewInt(int64(maxInt)), big.NewInt(10))
	bigB := new(big.Int).Add(big.NewInt(int64(maxInt)), big.NewInt(20))
	occA, err := OccursFromBig(bigA)
	if err != nil {
		t.Fatalf("OccursFromBig a error: %v", err)
	}
	occB, err := OccursFromBig(bigB)
	if err != nil {
		t.Fatalf("OccursFromBig b error: %v", err)
	}
	sumBig := AddOccurs(occA, occB)
	wantSum := new(big.Int).Add(bigA, bigB).String()
	if sumBig.String() != wantSum {
		t.Fatalf("unexpected sum: %s, want %s", sumBig, wantSum)
	}

	product := MulOccurs(OccursFromInt(maxInt), OccursFromInt(2))
	if product.IsUnbounded() {
		t.Fatalf("unexpected unbounded product")
	}
	if _, ok := product.Int(); ok {
		t.Fatalf("expected product to not fit in int")
	}
	if product.CmpInt(maxInt) <= 0 {
		t.Fatalf("expected product to exceed maxInt")
	}

	productBig := MulOccurs(occA, occB)
	wantProduct := new(big.Int).Mul(bigA, bigB).String()
	if productBig.String() != wantProduct {
		t.Fatalf("unexpected product: %s, want %s", productBig, wantProduct)
	}

	if got := MulOccurs(OccursFromInt(0), OccursUnbounded); !got.IsZero() {
		t.Fatalf("expected zero product")
	}
}

func TestOccursMinMaxWithBig(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	bigValue := new(big.Int).Add(big.NewInt(int64(maxInt)), big.NewInt(1))
	occBig, err := OccursFromBig(bigValue)
	if err != nil {
		t.Fatalf("OccursFromBig error: %v", err)
	}

	if got := MinOccurs(OccursUnbounded, occBig); !got.Equal(occBig) {
		t.Fatalf("expected min to pick bounded value")
	}
	if got := MaxOccurs(OccursUnbounded, occBig); !got.IsUnbounded() {
		t.Fatalf("expected max to pick unbounded")
	}

	small := OccursFromInt(1)
	if got := MinOccurs(small, occBig); !got.Equal(small) {
		t.Fatalf("expected min to pick small")
	}
	if got := MaxOccurs(small, occBig); !got.Equal(occBig) {
		t.Fatalf("expected max to pick big")
	}
}

func TestOccursCmpBig(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	bigValue := new(big.Int).Add(big.NewInt(int64(maxInt)), big.NewInt(1))
	occBig, err := OccursFromBig(bigValue)
	if err != nil {
		t.Fatalf("OccursFromBig error: %v", err)
	}
	occSmall := OccursFromInt(maxInt)

	if occBig.Cmp(occSmall) <= 0 {
		t.Fatalf("expected big > small")
	}
	if occSmall.Cmp(occBig) >= 0 {
		t.Fatalf("expected small < big")
	}
	if occBig.Cmp(OccursUnbounded) >= 0 {
		t.Fatalf("expected big < unbounded")
	}
	if OccursUnbounded.Cmp(occBig) <= 0 {
		t.Fatalf("expected unbounded > big")
	}
	if occBig.CmpInt(maxInt) <= 0 {
		t.Fatalf("expected big to exceed maxInt")
	}

	occBig2, err := OccursFromBig(bigValue)
	if err != nil {
		t.Fatalf("OccursFromBig error: %v", err)
	}
	if occBig.Cmp(occBig2) != 0 {
		t.Fatalf("expected big values to compare equal")
	}
}
