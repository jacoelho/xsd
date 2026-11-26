package xmlstream

import (
	"errors"
	"strings"
	"testing"
)

func TestDOCTYPERejectedAtDeclarationStart(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		input        string
		line, column int
	}{
		{name: "before the root", input: "\n<?p x?><!DOCTYPE r><r/>", line: 2, column: 8},
		{name: "after the root", input: "<r/><!DOCTYPE r>", line: 1, column: 5},
		{name: "inside content", input: "<r>\n  <!DOCTYPE r></r>", line: 2, column: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var reader Reader
			defer reader.Detach()
			if err := reader.Reset(strings.NewReader(test.input), Config{EmitPI: true}); err != nil {
				t.Fatal(err)
			}
			err := drainReader(&reader)
			if feature, ok := UnsupportedFeatureOf(err); !ok || feature != UnsupportedDTD {
				t.Fatalf("UnsupportedFeatureOf(%v) = %v, %t; want UnsupportedDTD", err, feature, ok)
			}
			if !errors.Is(err, errUnsupportedDTD) {
				t.Errorf("error %v does not wrap errUnsupportedDTD", err)
			}
			if line, column := reader.ErrorPos(err); line != test.line || column != test.column {
				t.Errorf("ErrorPos = %d:%d, want %d:%d", line, column, test.line, test.column)
			}
		})
	}
}

func TestDOCTYPERejectionReturnsNoToken(t *testing.T) {
	t.Parallel()
	var reader Reader
	defer reader.Detach()
	if err := reader.Reset(strings.NewReader("\n<?p x?><!DOCTYPE r><r/>"), Config{EmitPI: true}); err != nil {
		t.Fatal(err)
	}
	for tok := mustNextToken(t, &reader); tok.Kind != KindPI; tok = mustNextToken(t, &reader) {
		if tok.Kind != KindCharData {
			t.Fatalf("token kind = %v before the PI", tok.Kind)
		}
	}
	tok, err := reader.Next()
	if tok != nil || err == nil {
		t.Fatalf("Next() = %v, %v; want no token and an error", tok, err)
	}
}
