package xmltext

import (
	"errors"
	"testing"
)

func TestParseEntityRefErrors(t *testing.T) {
	if _, _, _, _, err := parseEntityRef([]byte("&"), 0, nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("parseEntityRef short error = %v, want %v", err, errInvalidEntity)
	}
	if _, _, _, _, err := parseEntityRef([]byte("&x"), 0, nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("parseEntityRef no-semi error = %v, want %v", err, errInvalidEntity)
	}
	if _, _, _, _, err := parseEntityRef([]byte("&;"), 0, nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("parseEntityRef empty error = %v, want %v", err, errInvalidEntity)
	}

	resolver := &entityResolver{custom: map[string]string{"ok": "v", "bad": "\x00"}}
	consumed, replacement, _, isNumeric, err := parseEntityRef([]byte("&ok;"), 0, resolver)
	if err != nil {
		t.Fatalf("parseEntityRef custom error = %v", err)
	}
	if consumed == 0 || isNumeric || replacement != "v" {
		t.Fatalf("parseEntityRef custom = %d/%v/%q, want consumed and v", consumed, isNumeric, replacement)
	}
	if _, _, _, _, err := parseEntityRef([]byte("&bad;"), 0, resolver); !errors.Is(err, errInvalidChar) {
		t.Fatalf("parseEntityRef bad error = %v, want %v", err, errInvalidChar)
	}

	consumed, _, r, isNumeric, err := parseEntityRef([]byte("&#x41;"), 0, nil)
	if err != nil {
		t.Fatalf("parseEntityRef numeric error = %v", err)
	}
	if !isNumeric || r != 'A' || consumed != len("&#x41;") {
		t.Fatalf("parseEntityRef numeric = %d/%v/%q, want A", consumed, isNumeric, r)
	}
}

func TestParseNumericEntityErrors(t *testing.T) {
	tests := [][]byte{
		[]byte("#"),
		[]byte("#x"),
		[]byte("#xG"),
		[]byte("#-1"),
	}
	for _, tt := range tests {
		if _, err := parseNumericEntity(tt); !errors.Is(err, errInvalidCharRef) {
			t.Fatalf("parseNumericEntity(%q) = %v, want %v", tt, err, errInvalidCharRef)
		}
	}
}

func TestParseNumericEntity(t *testing.T) {
	if _, err := parseNumericEntity([]byte("#9")); err != nil {
		t.Fatalf("parseNumericEntity decimal error = %v", err)
	}
	if _, err := parseNumericEntity([]byte("#xA")); err != nil {
		t.Fatalf("parseNumericEntity hex error = %v", err)
	}
	if _, err := parseNumericEntity([]byte("#x110000")); err == nil {
		t.Fatalf("expected error for out of range value")
	}
}
