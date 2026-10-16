package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestListXMLWhitespaceAdmission(t *testing.T) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	id, ok := value.BuiltinTypeID("NMTOKENS")
	if !ok {
		t.Fatal("missing NMTOKENS")
	}
	for _, tc := range []struct {
		name  string
		input string
		error string
	}{
		{name: "XML separators", input: " \tα\rβ\n "},
		{name: "NBSP remains in field", input: "α\u00a0β", error: "invalid NMTOKEN"},
		{name: "invalid second field", input: "α @", error: "invalid NMTOKEN"},
	} {
		for _, mode := range []string{"string", "borrowed bytes", "projected bytes"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				var got value.Value
				var validationErr error
				switch mode {
				case "string":
					got, validationErr = program.Validate(id, tc.input, value.Resolver{}, value.NeedCanonical, 64, nil)
				case "borrowed bytes":
					got, validationErr = program.ValidateBytes(id, []byte(tc.input), value.Resolver{}, 0, 64, nil)
				case "projected bytes":
					got, validationErr = program.ValidateBytes(id, []byte(tc.input), value.Resolver{}, value.NeedCanonical, 64, nil)
				}
				if tc.error != "" {
					if validationErr == nil || validationErr.Error() != tc.error {
						t.Fatalf("validation error = %v, want %q", validationErr, tc.error)
					}
					return
				}
				if validationErr != nil {
					t.Fatal(validationErr)
				}
				if got.Type() != id {
					t.Fatalf("result type = %d, want %d", got.Type(), id)
				}
				if mode != "borrowed bytes" && got.CanonicalText() != "α β" {
					t.Fatalf("canonical = %q, want %q", got.CanonicalText(), "α β")
				}
			})
		}
	}
}

func TestBorrowedXMLWhitespacePrimitiveAdmission(t *testing.T) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		kind  string
		input string
		valid bool
	}{
		{name: "Boolean fallback", kind: "boolean", input: "\ttrue\r", valid: true},
		{name: "integer XML trim", kind: "integer", input: " \t+001\r\n", valid: true},
		{name: "integer NBSP retained", kind: "integer", input: "\u00a01\u00a0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, found := value.BuiltinTypeID(tc.kind)
			if !found {
				t.Fatalf("missing %s", tc.kind)
			}
			_, validationErr := program.ValidateBytes(id, []byte(tc.input), value.Resolver{}, 0, 64, nil)
			if (validationErr == nil) != tc.valid {
				t.Fatalf("validation error = %v, valid = %v", validationErr, tc.valid)
			}
		})
	}
}
