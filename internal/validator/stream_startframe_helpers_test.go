package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func attrsForDoc(t *testing.T, docXML string) attributeIndex {
	t.Helper()

	dec, err := xmlstream.NewStringReader(strings.NewReader(docXML))
	if err != nil {
		t.Fatalf("NewStringReader: %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.Kind != xmlstream.EventStartElement {
		t.Fatalf("expected start element, got %v", ev.Kind)
	}
	return newAttributeIndex(ev.Attrs)
}

func TestValidateNilAttribute(t *testing.T) {
	tests := []struct {
		name      string
		docXML    string
		nillable  bool
		wantNil   bool
		wantError errors.ErrorCode
	}{
		{
			name:      "invalid boolean",
			docXML:    `<e xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="maybe"/>`,
			nillable:  true,
			wantNil:   false,
			wantError: errors.ErrDatatypeInvalid,
		},
		{
			name:      "not nillable",
			docXML:    `<e xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`,
			nillable:  false,
			wantNil:   false,
			wantError: errors.ErrElementNotNillable,
		},
		{
			name:      "nillable true",
			docXML:    `<e xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`,
			nillable:  true,
			wantNil:   true,
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := attrsForDoc(t, tt.docXML)
			run := &streamRun{validationRun: &validationRun{}}
			decl := &grammar.CompiledElement{Nillable: tt.nillable}

			isNil, violations := run.validateNilAttribute(attrs, decl, "e")
			if tt.wantError == "" {
				if len(violations) > 0 {
					t.Fatalf("unexpected violations: %v", violations)
				}
				if isNil != tt.wantNil {
					t.Fatalf("isNil = %v, want %v", isNil, tt.wantNil)
				}
				return
			}

			if len(violations) != 1 {
				t.Fatalf("expected 1 violation, got %d", len(violations))
			}
			if violations[0].Code != string(tt.wantError) {
				t.Fatalf("violation code = %q, want %q", violations[0].Code, tt.wantError)
			}
		})
	}
}

func TestResolveEffectiveType(t *testing.T) {
	run := &streamRun{validationRun: &validationRun{}}
	ev := &streamStart{Name: types.QName{Local: "e"}}
	attrs := attributeIndex{}

	abstractType := &grammar.CompiledType{Abstract: true, QName: types.QName{Local: "T"}}
	decl := &grammar.CompiledElement{Type: abstractType}
	effectiveType, violations := run.resolveEffectiveType(ev, attrs, decl)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Code != string(errors.ErrElementTypeAbstract) {
		t.Fatalf("violation code = %q, want %q", violations[0].Code, errors.ErrElementTypeAbstract)
	}
	if effectiveType != nil {
		t.Fatalf("expected nil effective type, got %v", effectiveType)
	}

	plainType := &grammar.CompiledType{QName: types.QName{Local: "Plain"}}
	decl = &grammar.CompiledElement{Type: plainType}
	effectiveType, violations = run.resolveEffectiveType(ev, attrs, decl)
	if len(violations) > 0 {
		t.Fatalf("unexpected violations: %v", violations)
	}
	if effectiveType != plainType {
		t.Fatalf("effectiveType = %v, want %v", effectiveType, plainType)
	}
}
