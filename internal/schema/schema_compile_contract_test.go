package schema_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func expectCategoryCode(t *testing.T, err error, category xsderrors.Category, code xsderrors.Code) {
	t.Helper()
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error %v is not *xsderrors.Error", err)
	}
	if x.Category() != category || x.Code() != code {
		t.Fatalf("error = (%s, %s), want (%s, %s): %v", x.Category(), x.Code(), category, code, err)
	}
}

const rootContentModelName = "r"
