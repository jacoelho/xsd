package xsd

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/xsderrors"
)

func TestClassifyPublicErrors(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		name string
		err  error
		fn   func(error) error
		kind ErrorKind
		code ErrorCode
	}{
		{
			name: "caller",
			err:  fmt.Errorf("compile schema: %w", cause),
			fn:   classifyCallerError,
			kind: KindCaller,
			code: ErrCaller,
		},
		{
			name: "io",
			err:  fmt.Errorf("open schema.xsd: %w", fs.ErrNotExist),
			fn:   classifyIOError,
			kind: KindIO,
			code: ErrIO,
		},
		{
			name: "schema parse",
			err: fmt.Errorf("load parsed schema: %w", &schemaast.ParseError{
				Code:    "schema-parse-error",
				Message: "parse XML",
				Err:     cause,
			}),
			fn:   classifySchemaError,
			kind: KindSchema,
			code: ErrSchemaParse,
		},
		{
			name: "schema semantic",
			err:  fmt.Errorf("prepare schema: %w", cause),
			fn:   classifySchemaError,
			kind: KindSchema,
			code: ErrSchemaSemantic,
		},
		{
			name: "internal",
			err:  fmt.Errorf("runtime build: %w", cause),
			fn:   classifyInternalError,
			kind: KindInternal,
			code: ErrInternal,
		},
		{
			name: "validation internal",
			err:  fmt.Errorf("validate document: %w", cause),
			fn:   classifyValidationInternalError,
			kind: KindInternal,
			code: ErrValidationInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(tt.err)
			var xsdErr Error
			if !errors.As(err, &xsdErr) {
				t.Fatalf("classified error = %T, want xsd.Error", err)
			}
			if xsdErr.Kind != tt.kind || xsdErr.Code != tt.code {
				t.Fatalf("classified error = {%v %v}, want {%v %v}", xsdErr.Kind, xsdErr.Code, tt.kind, tt.code)
			}
			if !errors.Is(err, tt.err) {
				t.Fatalf("classified error does not preserve cause %v", tt.err)
			}
			if !errors.Is(err, Error{Kind: tt.kind, Code: tt.code}) {
				t.Fatalf("errors.Is(%v, target {%v %v}) = false", err, tt.kind, tt.code)
			}
		})
	}
}

func TestClassifyPublicErrorPreservesExistingInternalCode(t *testing.T) {
	err := classifyInternalError(fmt.Errorf("wrapped: %w", xsderrors.NewKind(
		xsderrors.KindCaller,
		xsderrors.ErrSchemaNotLoaded,
		"schema not loaded",
	)))

	var xsdErr Error
	if !errors.As(err, &xsdErr) {
		t.Fatalf("classified error = %T, want xsd.Error", err)
	}
	if xsdErr.Kind != KindCaller || xsdErr.Code != ErrSchemaNotLoaded {
		t.Fatalf("classified error = {%v %v}, want {%v %v}", xsdErr.Kind, xsdErr.Code, KindCaller, ErrSchemaNotLoaded)
	}
}

func TestClassifyPublicErrorDoesNotWrapRootErrorInItself(t *testing.T) {
	err := Error{Kind: KindCaller, Code: ErrSchemaNotLoaded, Message: "schema not loaded"}
	got := classifyInternalError(err)

	var xsdErr Error
	if !errors.As(got, &xsdErr) {
		t.Fatalf("classified error = %T, want xsd.Error", got)
	}
	if xsdErr.Err != nil {
		t.Fatalf("classified root error unwrap = %T, want nil", xsdErr.Err)
	}
}
