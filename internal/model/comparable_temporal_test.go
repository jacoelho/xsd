package model

import (
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/num"
)

func TestSecondsToDuration(t *testing.T) {
	t.Parallel()

	t.Run("fractional seconds", func(t *testing.T) {
		seconds, parseErr := num.ParseDec([]byte("1.5"))
		if parseErr != nil {
			t.Fatalf("ParseDec() error = %v", parseErr)
		}
		got, err := SecondsToDuration(seconds)
		if err != nil {
			t.Fatalf("SecondsToDuration() error = %v", err)
		}
		if got != 1500*time.Millisecond {
			t.Fatalf("SecondsToDuration() = %v, want %v", got, 1500*time.Millisecond)
		}
	})

	t.Run("negative second value", func(t *testing.T) {
		_, err := SecondsToDuration(num.Dec{Sign: -1, Coef: []byte("1"), Scale: 0})
		if err == nil {
			t.Fatal("SecondsToDuration() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot be negative") {
			t.Fatalf("SecondsToDuration() error = %v, want substring %q", err, "cannot be negative")
		}
	})

	t.Run("second value too large", func(t *testing.T) {
		seconds, parseErr := num.ParseDec([]byte("999999999999999999999"))
		if parseErr != nil {
			t.Fatalf("ParseDec() error = %v", parseErr)
		}
		_, err := SecondsToDuration(seconds)
		if err == nil {
			t.Fatal("SecondsToDuration() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "second value too large") {
			t.Fatalf("SecondsToDuration() error = %v, want substring %q", err, "second value too large")
		}
	})
}
