package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestRecordIDStoresStableCopy(t *testing.T) {
	t.Parallel()

	sess := &Session{}
	value := []byte("alpha")
	if err := sess.recordID(value); err != nil {
		t.Fatalf("recordID() error = %v", err)
	}
	value[0] = 'z'

	err := sess.recordID([]byte("alpha"))
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDuplicateID {
		t.Fatalf("duplicate recordID() error = %v, want %s", err, xsderrors.ErrDuplicateID)
	}
}

func TestRecordIDRefStoresStableCopy(t *testing.T) {
	t.Parallel()

	sess := &Session{}
	ref := []byte("alpha")
	sess.recordIDRef(ref)
	ref[0] = 'z'

	if err := sess.recordID([]byte("alpha")); err != nil {
		t.Fatalf("recordID() error = %v", err)
	}
	if errs := sess.validateIDRefs(); len(errs) != 0 {
		t.Fatalf("validateIDRefs() errs = %v, want none", errs)
	}
}
