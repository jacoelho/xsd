package validate

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestNormalizeOptionsRejectsNegativeLimits(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "errors", opts: Options{MaxErrors: -1}},
		{name: "identity scopes", opts: Options{MaxIdentityScopes: -1}},
		{name: "identity entries", opts: Options{MaxIdentityEntries: -1}},
		{name: "identity tuple bytes", opts: Options{MaxIdentityTupleBytes: -1}},
		{name: "schema location namespaces", opts: Options{MaxSchemaLocationNamespaces: -1}},
		{name: "schema location namespace bytes", opts: Options{MaxSchemaLocationNamespaceBytes: -1}},
		{name: "instance depth", opts: Options{MaxInstanceDepth: -1}},
		{name: "instance attributes", opts: Options{MaxInstanceAttributes: -1}},
		{name: "instance text bytes", opts: Options{MaxInstanceTextBytes: -1}},
		{name: "instance token bytes", opts: Options{MaxInstanceTokenBytes: -1}},
		{name: "instance bytes", opts: Options{MaxInstanceBytes: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeOptions(tt.opts)
			if err == nil {
				t.Fatal("NormalizeOptions() succeeded")
			}
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("error type = %T, want *xsderrors.Error", err)
			}
			if xerr.Code != xsderrors.CodeValidationOption {
				t.Fatalf("code = %s, want %s", xerr.Code, xsderrors.CodeValidationOption)
			}
		})
	}
}

func TestNormalizeOptionsCopiesLimits(t *testing.T) {
	limits, err := NormalizeOptions(Options{
		MaxErrors:                       1,
		MaxIdentityScopes:               2,
		MaxIdentityEntries:              3,
		MaxIdentityTupleBytes:           4,
		MaxSchemaLocationNamespaces:     5,
		MaxSchemaLocationNamespaceBytes: 6,
		MaxInstanceDepth:                7,
		MaxInstanceAttributes:           8,
		MaxInstanceTextBytes:            9,
		MaxInstanceTokenBytes:           10,
		MaxInstanceBytes:                11,
	})
	if err != nil {
		t.Fatalf("NormalizeOptions() error = %v", err)
	}
	want := Limits{
		Errors:                       1,
		IdentityScopes:               2,
		IdentityEntries:              3,
		IdentityTupleBytes:           4,
		SchemaLocationNamespaces:     5,
		SchemaLocationNamespaceBytes: 6,
		InstanceDepth:                7,
		InstanceAttributes:           8,
		InstanceTextBytes:            9,
		InstanceTokenBytes:           10,
		InstanceBytes:                11,
	}
	if limits != want {
		t.Fatalf("limits = %+v, want %+v", limits, want)
	}
}

func TestNormalizeOptionsUsesFiniteDefaults(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := Limits{
		Errors:                       defaultMaxErrors,
		IdentityScopes:               defaultMaxIdentityScopes,
		IdentityEntries:              defaultMaxIdentityEntries,
		IdentityTupleBytes:           defaultMaxIdentityTupleBytes,
		SchemaLocationNamespaces:     defaultMaxSchemaLocationNamespaces,
		SchemaLocationNamespaceBytes: defaultMaxSchemaLocationNamespaceBytes,
		InstanceDepth:                defaultMaxInstanceDepth,
		InstanceAttributes:           defaultMaxInstanceAttributes,
		InstanceTextBytes:            defaultMaxInstanceTextBytes,
		InstanceTokenBytes:           defaultMaxInstanceTokenBytes,
		InstanceBytes:                defaultMaxInstanceBytes,
	}
	if limits != want {
		t.Fatalf("limits = %+v, want %+v", limits, want)
	}
}
