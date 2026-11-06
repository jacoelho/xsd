package schema

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaValueError(t *testing.T) {
	t.Parallel()

	unsupported := xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported regex", nil)
	located := xsderrors.WithLocation("constraint.xsd", 7, 11, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content work exhausted"))
	wrappedUnsupported := fmt.Errorf("mixed content constraint: %w", unsupported)

	tests := []struct {
		name     string
		err      error
		code     xsderrors.Code
		contains string
		same     bool
		internal bool
	}{
		{name: "metadata", err: fmt.Errorf("broken: %w", value.ErrMetadata), internal: true, contains: "subject: broken"},
		{name: "limit", err: fmt.Errorf("too much: %w", value.ErrLimit), code: xsderrors.CodeSchemaLimit, contains: "subject: too much"},
		{name: "pattern limit", err: value.ErrPatternLimit, code: xsderrors.CodeSchemaLimit, contains: "subject: MaxSchemaPatternWork=7 exceeded"},
		{name: "pattern total limit", err: value.ErrPatternTotalLimit, code: xsderrors.CodeSchemaLimit, contains: "subject: MaxSchemaTotalPatternWork=9 exceeded"},
		{name: "unsupported", err: unsupported, same: true},
		{name: "wrapped unsupported", err: wrappedUnsupported, same: true},
		{name: "located diagnostic", err: located, same: true},
		{name: "plain", err: errors.New("invalid integer"), code: xsderrors.CodeSchemaFacet, contains: "subject: invalid integer"},
		{name: "metadata outranks limit", err: errors.Join(value.ErrLimit, value.ErrMetadata), internal: true},
		{name: "limit outranks unsupported", err: errors.Join(value.ErrLimit, unsupported), code: xsderrors.CodeSchemaLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := schemaValueError(Limits{MaxSchemaPatternWork: 7, MaxSchemaTotalPatternWork: 9}, xsderrors.CodeSchemaFacet, "subject", tt.err)
			if tt.same {
				if !errors.Is(got, tt.err) {
					t.Fatalf("schemaValueError() = %v, want original error", got)
				}
				return
			}
			xerr, ok := errors.AsType[*xsderrors.Error](got)
			if !ok {
				t.Fatalf("schemaValueError() = %T %v, want *xsderrors.Error", got, got)
			}
			if tt.internal {
				if xerr.Category() != xsderrors.CategoryInternal {
					t.Fatalf("category = %s, want internal", xerr.Category())
				}
			} else if xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != tt.code {
				t.Fatalf("diagnostic = %s/%s, want schema compile %s", xerr.Category(), xerr.Code(), tt.code)
			}
			if !strings.Contains(xerr.Message(), tt.contains) {
				t.Fatalf("message = %q, want it to contain %q", xerr.Message(), tt.contains)
			}
		})
	}

	if err := schemaValueError(Limits{}, xsderrors.CodeSchemaFacet, "subject", nil); err != nil {
		t.Fatalf("schemaValueError(nil) = %v", err)
	}
}
