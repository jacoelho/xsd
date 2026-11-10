package validate

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaLocationHintsAcceptWellFormedHints(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ local, value string }{
		{vocab.XSIAttrSchemaLocation, "urn:a a.xsd\turn:b b.xsd"},
		// xsi:schemaLocation is a list of anyURI: any item count is valid.
		{vocab.XSIAttrSchemaLocation, "urn:a"},
		{vocab.XSIAttrSchemaLocation, "urn:a a.xsd urn:b"},
		{vocab.XSIAttrSchemaLocation, ""},
		// The anyURI lexical rule (internal/uriref) accepts non-ASCII UTF-8,
		// and NBSP is not XML whitespace, so this is one valid item.
		{vocab.XSIAttrSchemaLocation, "urn:t\u00a0hinted.xsd"},
		{vocab.XSIAttrNoNamespaceSchemaLocation, "\tno-ns.xsd\n"},
		{vocab.XSIAttrNoNamespaceSchemaLocation, ""},
		{vocab.XSIAttrNoNamespaceSchemaLocation, " \t\n\r"},
	} {
		if err := checkSchemaLocationHint(tc.local, tc.value); err != nil {
			t.Fatalf("%s=%q: %v", tc.local, tc.value, err)
		}
	}
}

func TestSchemaLocationHintsRejectMalformedHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		local   string
		value   string
		message string
	}{
		{
			name:    "schemaLocation invalid URI",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t %zz",
			message: "invalid xsi:schemaLocation URI \"%zz\"",
		},
		{
			name:    "schemaLocation invalid first item",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "%zz urn:t",
			message: "invalid xsi:schemaLocation URI \"%zz\"",
		},
		{
			name:    "noNamespace invalid URI",
			local:   vocab.XSIAttrNoNamespaceSchemaLocation,
			value:   "%zz",
			message: "invalid xsi:noNamespaceSchemaLocation URI \"%zz\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkSchemaLocationHint(tc.local, tc.value)
			expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
			expectXSDMessage(t, err, tc.message)
		})
	}
}

func expectXSDMessage(t *testing.T, err error, message string) {
	t.Helper()
	var x *xsderrors.Error
	if !errors.As(err, &x) {
		t.Fatalf("error = %v, want *xsderrors.Error", err)
	}
	if x.Message() != message {
		t.Fatalf("error message = %q, want %q", x.Message(), message)
	}
}
