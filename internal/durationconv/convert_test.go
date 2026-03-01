package durationconv

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
)

func TestParseToStdDuration(t *testing.T) {
	got, err := ParseToStdDuration("P2DT3H4M5.5S")
	if err != nil {
		t.Fatalf("ParseToStdDuration() error = %v", err)
	}
	want := 51*time.Hour + 4*time.Minute + 5500*time.Millisecond
	if got != want {
		t.Fatalf("ParseToStdDuration() = %v, want %v", got, want)
	}
}

func TestToStdDurationIndeterminate(t *testing.T) {
	_, err := ToStdDuration(durationlex.Duration{Years: 1})
	if !errors.Is(err, ErrIndeterminate) {
		t.Fatalf("ToStdDuration() error = %v, want ErrIndeterminate", err)
	}
}

func TestToStdDurationOverflow(t *testing.T) {
	_, err := ToStdDuration(durationlex.Duration{Days: int(^uint(0) >> 1)})
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("ToStdDuration() error = %v, want ErrOverflow", err)
	}
}

func TestToStdDurationComponentRange(t *testing.T) {
	_, err := ToStdDuration(durationlex.Duration{Days: -1})
	if !errors.Is(err, ErrComponentRange) {
		t.Fatalf("ToStdDuration() error = %v, want ErrComponentRange", err)
	}
}

func TestToStdDurationSecondsErrorPassthrough(t *testing.T) {
	seconds, parseErr := num.ParseDec([]byte("-1"))
	if parseErr != nil {
		t.Fatalf("ParseDec() error = %v", parseErr)
	}
	_, err := ToStdDuration(durationlex.Duration{Seconds: seconds})
	if err == nil {
		t.Fatal("ToStdDuration() expected error")
	}
	if !errors.Is(err, ErrComponentRange) {
		t.Fatalf("ToStdDuration() error = %v, want ErrComponentRange", err)
	}
}

func TestToStdDurationSecondsOverflowUsesSentinel(t *testing.T) {
	seconds, parseErr := num.ParseDec([]byte("9223372037"))
	if parseErr != nil {
		t.Fatalf("ParseDec() error = %v", parseErr)
	}
	_, err := ToStdDuration(durationlex.Duration{Seconds: seconds})
	if err == nil {
		t.Fatal("ToStdDuration() expected error")
	}
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("ToStdDuration() error = %v, want ErrOverflow", err)
	}
}

func TestToStdDurationSecondsPrecisionErrorIsNotOverflow(t *testing.T) {
	seconds, parseErr := num.ParseDec([]byte("0.1234567891"))
	if parseErr != nil {
		t.Fatalf("ParseDec() error = %v", parseErr)
	}
	_, err := ToStdDuration(durationlex.Duration{Seconds: seconds})
	if err == nil {
		t.Fatal("ToStdDuration() expected error")
	}
	if errors.Is(err, ErrOverflow) {
		t.Fatalf("ToStdDuration() error = %v, must not wrap ErrOverflow for precision errors", err)
	}
	if !strings.Contains(err.Error(), "precision exceeds") {
		t.Fatalf("ToStdDuration() error = %v, want precision error", err)
	}
}
