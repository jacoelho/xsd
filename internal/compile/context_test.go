package compile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jacoelho/xsd/xsderrors"
)

type cancelDuringSchemaPreflightContext struct {
	checks int
}

func (*cancelDuringSchemaPreflightContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*cancelDuringSchemaPreflightContext) Done() <-chan struct{}       { return nil }
func (c *cancelDuringSchemaPreflightContext) Err() error {
	c.checks++
	if c.checks > 1 {
		return context.Canceled
	}
	return nil
}
func (*cancelDuringSchemaPreflightContext) Value(any) any { return nil }

func TestParseRawSchemaDocumentClassifiesPreflightCancellation(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := new(cancelDuringSchemaPreflightContext)
	_, err = parseRawSchemaDocument(ctx, "schema.xsd", "schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), limits)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("parseRawSchemaDocument() error = %v, want cancellation cause", err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category != xsderrors.CategoryCanceled || diagnostic.Code != xsderrors.CodeCompileCanceled {
		t.Fatalf("parseRawSchemaDocument() error = %#v, want canceled/compile.canceled", diagnostic)
	}
}
