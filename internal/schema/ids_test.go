package schema

import (
	"errors"
	"strconv"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileIDAllocationDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		next func(int) error
		msg  string
	}{
		{name: "simple type", next: func(n int) error {
			_, err := nextSimpleTypeID(n)
			return err
		}, msg: "simple type limit exceeded"},
		{name: "complex type", next: func(n int) error {
			_, err := nextComplexTypeID(n)
			return err
		}, msg: "complex type limit exceeded"},
		{name: "element", next: func(n int) error {
			_, err := nextElementID(n)
			return err
		}, msg: "element declaration limit exceeded"},
		{name: "attribute", next: func(n int) error {
			_, err := nextAttributeID(n)
			return err
		}, msg: "attribute declaration limit exceeded"},
		{name: "content model", next: func(n int) error {
			_, err := nextContentModelID(n)
			return err
		}, msg: "content model limit exceeded"},
		{name: "attribute use set", next: func(n int) error {
			_, err := nextAttributeUseSetID(n)
			return err
		}, msg: "attribute use set limit exceeded"},
		{name: "wildcard", next: func(n int) error {
			_, err := nextWildcardID(n)
			return err
		}, msg: "wildcard limit exceeded"},
		{name: "identity constraint", next: func(n int) error {
			_, err := nextIdentityConstraintID(n)
			return err
		}, msg: "identity constraint limit exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.next(0); err != nil {
				t.Fatalf("next(0) error = %v", err)
			}
			err := tt.next(-1)
			expectSchemaLimitDiagnostic(t, err, tt.msg)
			if strconv.IntSize > 32 {
				err = tt.next(int(^uint32(0)))
				expectSchemaLimitDiagnostic(t, err, tt.msg)
			}
		})
	}
}

func TestCheckedUint32IndexDiagnostic(t *testing.T) {
	t.Parallel()

	id, err := checkedUint32Index(2, "index limit exceeded")
	if err != nil || id != 2 {
		t.Fatalf("CheckedUint32Index(2) = %d, %v, want 2, nil", id, err)
	}
	_, err = checkedUint32Index(-1, "index limit exceeded")
	expectSchemaLimitDiagnostic(t, err, "index limit exceeded")
}

func expectSchemaLimitDiagnostic(t *testing.T, err error, msg string) {
	t.Helper()

	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want *xsderrors.Error", err)
	}
	if x.Category() != xsderrors.CategorySchemaCompile || x.Code() != xsderrors.CodeSchemaLimit || x.Message() != msg {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", x.Category(), x.Code(), x.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit, msg)
	}
}

func TestCompileIDAllocationMatchesRuntimeIDs(t *testing.T) {
	t.Parallel()

	id, err := nextElementID(3)
	if err != nil {
		t.Fatalf("NextElementID(3) error = %v", err)
	}
	if id != ElementID(3) {
		t.Fatalf("NextElementID(3) = %d, want 3", id)
	}
}
