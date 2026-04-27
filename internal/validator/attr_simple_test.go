package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestValidateSimpleRejectsNonSpecialAttribute(t *testing.T) {
	t.Parallel()

	_, err := ValidateSimple(
		newRuntimeSchema(t),
		[]Start{{Sym: 11}},
		[]Class{ClassOther},
		false,
		nil,
		nil,
	)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrValidateSimpleTypeAttrNotAllowed {
		t.Fatalf("ValidateSimple() error = %v, want %s", err, xsderrors.ErrValidateSimpleTypeAttrNotAllowed)
	}
}

func TestValidateSimpleStoresAllowedAttributes(t *testing.T) {
	t.Parallel()

	var appended []Start
	got, err := ValidateSimple(
		newRuntimeSchema(t),
		[]Start{{Sym: 1}, {Sym: 2}},
		[]Class{ClassXSIKnown, ClassXML},
		true,
		nil,
		func(validated []Start, attr Start, store bool) []Start {
			appended = append(appended, attr)
			return append(validated, attr)
		},
	)
	if err != nil {
		t.Fatalf("ValidateSimple() error = %v", err)
	}
	if len(got) != 2 || len(appended) != 2 {
		t.Fatalf("got = %#v, appended = %#v, want two stored attrs", got, appended)
	}
}

func TestValidateSimpleSkipsStorageWhenDisabled(t *testing.T) {
	t.Parallel()

	called := false
	got, err := ValidateSimple(
		newRuntimeSchema(t),
		[]Start{{Sym: 1}},
		[]Class{ClassXML},
		false,
		nil,
		func(validated []Start, attr Start, store bool) []Start {
			called = true
			return validated
		},
	)
	if err != nil {
		t.Fatalf("ValidateSimple() error = %v", err)
	}
	if got != nil {
		t.Fatalf("got = %#v, want nil", got)
	}
	if called {
		t.Fatal("appendRaw was called when storage was disabled")
	}
}
