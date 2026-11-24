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
	located := xsderrors.WithLocation("constraint.xsd", 7, 11, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content work exhausted", nil))
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
				if !errors.Is(got, tt.err) {
					t.Fatalf("schemaValueError() = %v, want it to wrap %v", got, tt.err)
				}
			} else if xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != tt.code {
				t.Fatalf("diagnostic = %s/%s, want schema compile %s", xerr.Category(), xerr.Code(), tt.code)
			}
			if !strings.Contains(xerr.Error(), tt.contains) {
				t.Fatalf("error = %q, want it to contain %q", xerr.Error(), tt.contains)
			}
		})
	}

	if err := schemaValueError(Limits{}, xsderrors.CodeSchemaFacet, "subject", nil); err != nil {
		t.Fatalf("schemaValueError(nil) = %v", err)
	}
}

func TestCompileRuleErrorReportsBuildDefectsAsInternalInvariants(t *testing.T) {
	t.Parallel()

	defect := newBuildDefect("attribute wildcard restriction references invalid wildcard")
	metadata := fmt.Errorf("type 9: %w", value.ErrMetadata)
	located := xsderrors.WithLocation("a.xsd", 3, 4, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "missing type", nil))
	tests := []struct {
		name        string
		subject     string
		err         error
		wantCode    xsderrors.Code
		wantMessage string
		wantCause   error
	}{
		{name: "build defect", err: defect, wantCode: xsderrors.CodeInternalInvariant, wantCause: defect},
		{name: "wrapped build defect", subject: "attribute use", err: fmt.Errorf("restriction: %w", defect), wantCode: xsderrors.CodeInternalInvariant, wantMessage: "attribute use", wantCause: defect},
		{name: "value metadata", err: metadata, wantCode: xsderrors.CodeInternalInvariant, wantCause: value.ErrMetadata},
		{name: "schema violation", err: errors.New("attribute wildcard restriction weakens processContents"), wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMessage: "attribute wildcard restriction weakens processContents"},
		{name: "schema violation with subject", subject: "invalid default", err: errors.New("not an integer"), wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMessage: "invalid default: not an integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := compileRuleError(xsderrors.CodeSchemaInvalidAttribute, tt.subject, tt.err)
			diagnostic, ok := errors.AsType[*xsderrors.Error](got)
			if !ok || diagnostic.Code() != tt.wantCode || diagnostic.Message() != tt.wantMessage {
				t.Fatalf("compileRuleError(%v) = %v, want %s %q", tt.err, got, tt.wantCode, tt.wantMessage)
			}
			if tt.wantCause != nil && !errors.Is(got, tt.wantCause) {
				t.Fatalf("compileRuleError(%v) does not wrap %v", tt.err, tt.wantCause)
			}
		})
	}
	if got := compileRuleError(xsderrors.CodeSchemaInvalidAttribute, "", located); got != located { //nolint:errorlint // An existing diagnostic is returned as is.
		t.Fatalf("compileRuleError(diagnostic) = %v, want the diagnostic unchanged", got)
	}
	if got := compileRuleError(xsderrors.CodeSchemaInvalidAttribute, "", nil); got != nil {
		t.Fatalf("compileRuleError(nil) = %v", got)
	}
}

func TestInvalidAttributeErrorReportsCorruptValueConstraintTypeAsInternalInvariant(t *testing.T) {
	t.Parallel()

	err := validateValueConstraintIdentity(nil, value.BuiltinType(value.PrimitiveString), &valueConstraint{}, valueConstraintOwnerElementDeclaration)
	diagnostic, ok := errors.AsType[*xsderrors.Error](invalidAttributeError(err))
	if !ok || diagnostic.Code() != xsderrors.CodeInternalInvariant || !errors.Is(diagnostic, err) {
		t.Fatalf("invalidAttributeError(%v) = %v, want an internal invariant wrapping it", err, diagnostic)
	}
}
