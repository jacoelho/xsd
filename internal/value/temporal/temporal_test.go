package temporal

import "testing"

func TestEqual_TimeLeapSecondDistinct(t *testing.T) {
	leap, err := Parse(KindTime, []byte("23:59:60"))
	if err != nil {
		t.Fatalf("Parse(leap) error = %v", err)
	}
	midnight, err := Parse(KindTime, []byte("00:00:00"))
	if err != nil {
		t.Fatalf("Parse(midnight) error = %v", err)
	}
	if Equal(leap, midnight) {
		t.Fatalf("expected leap second to differ from midnight")
	}
}

func TestEqual_DateTimeLeapSecondDistinct(t *testing.T) {
	leap, err := Parse(KindDateTime, []byte("1999-12-31T23:59:60Z"))
	if err != nil {
		t.Fatalf("Parse(leap) error = %v", err)
	}
	nextSecond, err := Parse(KindDateTime, []byte("2000-01-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("Parse(nextSecond) error = %v", err)
	}
	if Equal(leap, nextSecond) {
		t.Fatalf("expected leap second to differ from next second")
	}
}

func TestCanonical_PreservesLeapSecond(t *testing.T) {
	leap, err := Parse(KindTime, []byte("23:59:60Z"))
	if err != nil {
		t.Fatalf("Parse(leap) error = %v", err)
	}
	if got := Canonical(leap); got != "23:59:60Z" {
		t.Fatalf("Canonical(leap) = %q, want %q", got, "23:59:60Z")
	}
}

func TestCanonical_TimeLeapWithOffsetProducesParseableCanonical(t *testing.T) {
	leap, err := Parse(KindTime, []byte("23:59:60+02:00"))
	if err != nil {
		t.Fatalf("Parse(leap) error = %v", err)
	}
	if got := Canonical(leap); got != "22:00:00Z" {
		t.Fatalf("Canonical(leap) = %q, want %q", got, "22:00:00Z")
	}
	if _, err := Parse(KindTime, []byte(Canonical(leap))); err != nil {
		t.Fatalf("Parse(Canonical(leap)) error = %v", err)
	}
}

func TestCanonical_DateTimeLeapWithOffsetProducesParseableCanonical(t *testing.T) {
	leap, err := Parse(KindDateTime, []byte("1999-12-31T23:59:60+02:00"))
	if err != nil {
		t.Fatalf("Parse(leap) error = %v", err)
	}
	if got := Canonical(leap); got != "1999-12-31T22:00:00Z" {
		t.Fatalf("Canonical(leap) = %q, want %q", got, "1999-12-31T22:00:00Z")
	}
	if _, err := Parse(KindDateTime, []byte(Canonical(leap))); err != nil {
		t.Fatalf("Parse(Canonical(leap)) error = %v", err)
	}
}

func TestCompare_IndeterminateWhenTimezoneMissing(t *testing.T) {
	withTZ, err := Parse(KindDateTime, []byte("2000-01-01T12:00:00Z"))
	if err != nil {
		t.Fatalf("Parse(withTZ) error = %v", err)
	}
	withoutTZ, err := Parse(KindDateTime, []byte("2000-01-01T12:00:00"))
	if err != nil {
		t.Fatalf("Parse(withoutTZ) error = %v", err)
	}
	if _, err := Compare(withTZ, withoutTZ); err == nil {
		t.Fatalf("expected indeterminate comparison error")
	}
}
