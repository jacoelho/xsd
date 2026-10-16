package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

const entityUnsupportedMessage = "ENTITY requires DTD entity declarations, which are not supported"

func TestBuiltinLexicalDiagnostics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, lexical, message string
		kind                   value.BuiltinKind
		code                   xsderrors.Code
	}{
		{name: "Name", kind: value.BuiltinName, lexical: "7item", message: "invalid Name"},
		{name: "NCName", kind: value.BuiltinNCName, lexical: "p:item", message: "invalid NCName"},
		{name: "ENTITY lexical", kind: value.BuiltinEntity, lexical: "p:item", message: "invalid NCName"},
		{name: "ENTITY unsupported", kind: value.BuiltinEntity, lexical: "item", message: entityUnsupportedMessage, code: xsderrors.CodeUnsupportedEntity},
		{name: "NMTOKEN", kind: value.BuiltinNMTOKEN, lexical: "two words", message: "invalid NMTOKEN"},
		{name: "language empty", kind: value.BuiltinLanguage, message: "invalid language"},
		{name: "xml lang empty", kind: value.BuiltinXMLLang},
		{name: "xml lang invalid", kind: value.BuiltinXMLLang, lexical: "en GB", message: "invalid language"},
		{name: "xml space", kind: value.BuiltinXMLSpace, lexical: "Default", message: "invalid xml:space"},
		{name: "integer decimal", kind: value.BuiltinInteger, lexical: "1.0", message: "invalid integer"},
		{name: "none", kind: value.BuiltinNone, lexical: "@"},
		{name: "unknown", kind: value.BuiltinKind(255), lexical: "@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := value.ValidateBuiltinDerived(value.BuiltinDerivedInput{Kind: tc.kind, Norm: tc.lexical})
			assertBuiltinDiagnostic(t, err, tc.message, tc.code)
		})
	}
}

func assertBuiltinDiagnostic(t *testing.T, err error, message string, code xsderrors.Code) {
	t.Helper()
	if message == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	if err == nil {
		t.Fatalf("accepted value; want %q", message)
	}
	var diagnostic *xsderrors.Error
	if code != "" {
		if !errors.As(err, &diagnostic) {
			t.Fatalf("error = %v, want structured unsupported error", err)
		}
		if diagnostic.Category() != xsderrors.CategoryUnsupported || diagnostic.Code() != code || diagnostic.Message() != message || diagnostic.Cause() != nil {
			t.Fatalf("unsupported diagnostic = %#v", diagnostic)
		}
		return
	}
	if err.Error() != message || errors.As(err, &diagnostic) {
		t.Fatalf("error = %v, want plain %q", err, message)
	}
}

func TestBuiltinAdmissionAndNormalization(t *testing.T) {
	t.Parallel()
	program, sealErr := value.NewBuilder(value.BuilderOptions{}).Seal()
	if sealErr != nil {
		t.Fatal(sealErr)
	}
	entity, found := value.BuiltinTypeID("ENTITY")
	if !found {
		t.Fatal("missing ENTITY")
	}
	for _, tc := range []struct{ lexical, message string }{
		{"\xff", "invalid UTF-8 value"},
		{"a\x00b", "invalid XML character"},
		{"a\x00\xff", "invalid UTF-8 value"},
	} {
		_, err := program.Validate(entity, tc.lexical, value.Resolver{}, 0, 16<<20, nil)
		assertBuiltinDiagnostic(t, err, tc.message, "")
	}
	space, found := value.BuiltinTypeID("xml:space")
	if !found {
		t.Fatal("missing xml:space")
	}
	got, err := program.Validate(space, " \tdefault\n ", value.Resolver{}, value.NeedCanonical, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.CanonicalText() != "default" {
		t.Fatalf("canonical = %q, want default", got.CanonicalText())
	}
}

func TestBuiltinEntityUnionFailureAndFallback(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	entity, found := value.BuiltinTypeID("ENTITY")
	if !found {
		t.Fatal("missing ENTITY")
	}
	decimal := value.BuiltinType(value.PrimitiveDecimal)
	stringID := value.BuiltinType(value.PrimitiveString)
	var unions [2]value.TypeID
	for i, later := range []value.TypeID{decimal, stringID} {
		id, reserveErr := builder.Reserve()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		if err := builder.Complete(id, value.TypeSpec{
			Variety: value.Union, Union: []value.TypeID{entity, later},
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: value.NoType, ListItem: value.NoType,
		}); err != nil {
			t.Fatal(err)
		}
		unions[i] = id
	}
	program, sealErr := builder.Seal()
	if sealErr != nil {
		t.Fatal(sealErr)
	}
	_, err := program.Validate(unions[0], "name", value.Resolver{}, 0, 16<<20, nil)
	assertBuiltinDiagnostic(t, err, entityUnsupportedMessage, xsderrors.CodeUnsupportedEntity)
	for _, bytesInput := range []bool{false, true} {
		var got value.Value
		var validationErr error
		if bytesInput {
			got, validationErr = program.ValidateBytes(unions[1], []byte("name"), value.Resolver{}, value.NeedCanonical, 16<<20, nil)
		} else {
			got, validationErr = program.Validate(unions[1], "name", value.Resolver{}, value.NeedCanonical, 16<<20, nil)
		}
		if validationErr != nil {
			t.Fatal(validationErr)
		}
		if got.Type() != unions[1] || got.SelectedType() != stringID || got.CanonicalText() != "name" {
			t.Fatalf("bytes=%t: type/selected/canonical = %d/%d/%q", bytesInput, got.Type(), got.SelectedType(), got.CanonicalText())
		}
	}
}
