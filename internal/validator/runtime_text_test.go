package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestConsumeTextElementOnlyWhitespace(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	var state TextState
	sess.ResetText(&state)

	if err := sess.ConsumeText(&state, runtime.ContentElementOnly, false, false, []byte(" \n\t")); err != nil {
		t.Fatalf("ConsumeText whitespace: %v", err)
	}
	if !state.HasText {
		t.Fatalf("expected HasText true")
	}
	if state.HasNonWS {
		t.Fatalf("expected HasNonWS false")
	}

	sess.ResetText(&state)
	if err := sess.ConsumeText(&state, runtime.ContentElementOnly, false, false, []byte("x")); err == nil {
		t.Fatalf("expected element-only text error")
	}
}

func TestConsumeTextMixedAllowsText(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	var state TextState
	sess.ResetText(&state)

	if err := sess.ConsumeText(&state, runtime.ContentMixed, false, false, []byte("text")); err != nil {
		t.Fatalf("ConsumeText mixed: %v", err)
	}
	if !state.HasNonWS {
		t.Fatalf("expected HasNonWS true")
	}
}

func TestConsumeTextSimpleCollects(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	var state TextState
	sess.ResetText(&state)

	if err := sess.ConsumeText(&state, runtime.ContentSimple, false, false, []byte("ab")); err != nil {
		t.Fatalf("ConsumeText simple: %v", err)
	}
	if err := sess.ConsumeText(&state, runtime.ContentSimple, false, false, []byte("cd")); err != nil {
		t.Fatalf("ConsumeText simple: %v", err)
	}
	if got := string(sess.TextSlice(state)); got != "abcd" {
		t.Fatalf("text = %q, want %q", got, "abcd")
	}
}

func TestConsumeTextNilledRejects(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	var state TextState
	sess.ResetText(&state)

	if err := sess.ConsumeText(&state, runtime.ContentMixed, false, true, []byte(" ")); err == nil {
		t.Fatalf("expected nilled text error")
	}
}

func TestConsumeTextAllMixed(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	var state TextState
	sess.ResetText(&state)

	if err := sess.ConsumeText(&state, runtime.ContentAll, true, false, []byte("text")); err != nil {
		t.Fatalf("ConsumeText all mixed: %v", err)
	}
}

func TestConsumeTextEmptyRejectsWhitespace(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	var state TextState
	sess.ResetText(&state)

	if err := sess.ConsumeText(&state, runtime.ContentEmpty, false, false, []byte(" \n\t")); err == nil {
		t.Fatalf("expected empty content to reject whitespace")
	}
}
