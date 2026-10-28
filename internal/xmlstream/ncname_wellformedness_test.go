package xmlstream

import (
	"errors"
	"strings"
	"testing"
)

// Namespaces in XML §7: processing instruction targets and entity names
// contain no colon.
func TestReaderRejectsColonInPITargets(t *testing.T) {
	t.Parallel()

	positions := []struct {
		name   string
		layout func(pi string) string
	}{
		{name: "prolog", layout: func(pi string) string { return pi + `<r/>` }},
		{name: "body", layout: func(pi string) string { return `<r>` + pi + `</r>` }},
		{name: "epilog", layout: func(pi string) string { return `<r/>` + pi }},
	}
	for _, target := range []struct {
		name  string
		valid bool
	}{
		{name: "a:b"}, {name: ":a"}, {name: "a:"}, {name: "xml:foo"}, {name: "é:b"},
		{name: "xml-stylesheet", valid: true}, {name: "a_b", valid: true}, {name: "é", valid: true}, {name: "a.b", valid: true},
	} {
		for _, position := range positions {
			for _, pi := range []string{`<?` + target.name + `?>`, `<?` + target.name + ` data?>`} {
				for _, emit := range []bool{false, true} {
					t.Run(target.name+"/"+position.name+"/"+pi, func(t *testing.T) {
						t.Parallel()
						var reader Reader
						if err := reader.Reset(strings.NewReader(position.layout(pi)), Config{EmitPI: emit}); err != nil {
							t.Fatal(err)
						}
						err := drainReader(&reader)
						if target.valid {
							if err != nil {
								t.Fatalf("drain(EmitPI=%v) error = %v", emit, err)
							}
							return
						}
						var xmlErr *Error
						if !errors.As(err, &xmlErr) || xmlErr.Kind != ErrorSyntax {
							t.Fatalf("drain(EmitPI=%v) error = %v, want syntax error", emit, err)
						}
					})
				}
			}
		}
	}
}

func TestEntityNamesWithColonAreMalformed(t *testing.T) {
	t.Parallel()

	for _, document := range []string{
		`<r>&a:b;</r>`, `<r a="&a:b;"/>`, `<r>&:a;</r>`, `<r>&a:;</r>`,
	} {
		t.Run(document, func(t *testing.T) {
			t.Parallel()
			var reader Reader
			if err := reader.Reset(strings.NewReader(document), Config{}); err != nil {
				t.Fatal(err)
			}
			err := drainReader(&reader)
			var xmlErr *Error
			if !errors.As(err, &xmlErr) || xmlErr.Kind != ErrorSyntax {
				t.Fatalf("drain() error = %v, want syntax error", err)
			}
		})
	}
	for _, entity := range []string{"a:b", ":a", "a:"} {
		if _, err := DecodeEntityReferenceString(entity); err == nil || IsUnsupportedEntityReference(err) {
			t.Fatalf("DecodeEntityReferenceString(%q) error = %v, want malformed entity", entity, err)
		}
	}
	if _, err := DecodeEntityReferenceString("missing"); !IsUnsupportedEntityReference(err) {
		t.Fatalf("DecodeEntityReferenceString(missing) error = %v, want unsupported entity", err)
	}
}
