package internalcore

import (
	"errors"
	"testing"
)

func TestParseValueForTypeDispatchesParser(t *testing.T) {
	t.Parallel()

	got, err := ParseValueForType("v", "string", struct{}{}, map[string]ValueParserFunc{
		"string": func(lexical string, _ any) (any, error) {
			return lexical + "-ok", nil
		},
	})
	if err != nil {
		t.Fatalf("ParseValueForType() error = %v", err)
	}
	if got != "v-ok" {
		t.Fatalf("ParseValueForType() = %v, want %v", got, "v-ok")
	}
}

func TestParseValueForTypeUnknownType(t *testing.T) {
	t.Parallel()

	_, err := ParseValueForType("v", "unknown", struct{}{}, map[string]ValueParserFunc{})
	if err == nil {
		t.Fatalf("expected error for unknown parser")
	}
}

func TestParseValueForTypePropagatesParserError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("parse failed")
	_, err := ParseValueForType("v", "string", struct{}{}, map[string]ValueParserFunc{
		"string": func(string, any) (any, error) {
			return nil, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ParseValueForType() error = %v, want %v", err, wantErr)
	}
}
