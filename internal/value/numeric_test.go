package value

import (
	"math"
	"testing"
)

func TestParseBoolean(t *testing.T) {
	if got, err := ParseBoolean([]byte(" true ")); err != nil || got != true {
		t.Fatalf("ParseBoolean(true) = %v, %v", got, err)
	}
	if got, err := ParseBoolean([]byte("0")); err != nil || got != false {
		t.Fatalf("ParseBoolean(0) = %v, %v", got, err)
	}
	if _, err := ParseBoolean([]byte("yes")); err == nil {
		t.Fatalf("expected error for invalid boolean")
	}
}

func TestParseFloat(t *testing.T) {
	if got, err := ParseFloat([]byte("INF")); err != nil || !math.IsInf(float64(got), 1) {
		t.Fatalf("ParseFloat(INF) = %v, %v", got, err)
	}
	if _, err := ParseFloat([]byte("+INF")); err == nil {
		t.Fatalf("expected error for +INF")
	}
}

func TestParseDouble(t *testing.T) {
	if got, err := ParseDouble([]byte("NaN")); err != nil || !math.IsNaN(got) {
		t.Fatalf("ParseDouble(NaN) = %v, %v", got, err)
	}
}
