package compile

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestInvalidAttributeError(t *testing.T) {
	t.Parallel()

	if err := invalidAttributeError(nil); err != nil {
		t.Fatalf("invalidAttributeError(nil) error = %v", err)
	}
	err := invalidAttributeError(fmt.Errorf("runtime reject"))
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("invalidAttributeError(reject) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaInvalidAttribute {
		t.Fatalf("invalidAttributeError diagnostic = %s/%s, want schema compile invalid attribute", xerr.Category, xerr.Code)
	}
	if xerr.Message != "runtime reject" {
		t.Fatalf("invalidAttributeError message = %q, want runtime message", xerr.Message)
	}
}
