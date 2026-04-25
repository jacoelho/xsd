package validator

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestApplyDefaultsRequiredMissing(t *testing.T) {
	t.Parallel()

	uses := []runtime.AttrUse{{Use: runtime.AttrRequired}}
	_, err := ApplyDefaults(
		uses,
		nil,
		false,
		false,
		nil,
		func(*runtime.AttrUse) Selection { return Selection{} },
		nil,
		func(runtime.ValueRef) []byte { return nil },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID) error { return nil },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID, runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
			return runtime.VKInvalid, nil, nil
		},
		nil,
	)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrRequiredAttributeMissing {
		t.Fatalf("ApplyDefaults() error = %v, want %s", err, xsderrors.ErrRequiredAttributeMissing)
	}
}

func TestApplyDefaultsDuplicateID(t *testing.T) {
	t.Parallel()

	uses := []runtime.AttrUse{{Validator: 7}}
	_, err := ApplyDefaults(
		uses,
		nil,
		false,
		true,
		nil,
		func(*runtime.AttrUse) Selection {
			return Selection{Present: true, Value: runtime.ValueRef{Present: true}}
		},
		func(runtime.ValidatorID) bool { return true },
		func(runtime.ValueRef) []byte { return []byte("id") },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID) error { return nil },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID, runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
			return runtime.VKInvalid, nil, nil
		},
		nil,
	)
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrMultipleIDAttr {
		t.Fatalf("ApplyDefaults() error = %v, want %s", err, xsderrors.ErrMultipleIDAttr)
	}
}

func TestApplyDefaultsStoresKeysAndTracksValues(t *testing.T) {
	t.Parallel()

	uses := []runtime.AttrUse{{
		Name:      11,
		Validator: 21,
	}}
	selection := Selection{
		Present: true,
		Fixed:   true,
		Value:   runtime.ValueRef{Present: true, Off: 3, Len: 4},
		Key:     runtime.ValueKeyRef{Kind: runtime.VKString},
		Member:  31,
	}

	var tracked struct {
		validator runtime.ValidatorID
		member    runtime.ValidatorID
		canonical string
	}
	materializeCalls := 0
	applied, err := ApplyDefaults(
		uses,
		nil,
		true,
		false,
		make([]Applied, 0, 1),
		func(*runtime.AttrUse) Selection { return selection },
		func(runtime.ValidatorID) bool { return false },
		func(runtime.ValueRef) []byte { return []byte("canon") },
		func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) error {
			tracked.validator = validator
			tracked.member = member
			tracked.canonical = string(canonical)
			return nil
		},
		func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID, stored runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
			materializeCalls++
			if validator != 21 || string(canonical) != "canon" || member != 31 || stored.Kind != runtime.VKString {
				t.Fatalf("unexpected materialize inputs: validator=%d canonical=%q member=%d kind=%d", validator, canonical, member, stored.Kind)
			}
			return runtime.VKString, []byte("derived"), nil
		},
		func(key []byte) []byte { return append([]byte("stored:"), key...) },
	)
	if err != nil {
		t.Fatalf("ApplyDefaults() error = %v", err)
	}
	if materializeCalls != 1 {
		t.Fatalf("materializeCalls = %d, want 1", materializeCalls)
	}
	if tracked.validator != 21 || tracked.member != 31 || tracked.canonical != "canon" {
		t.Fatalf("tracked = %+v, want validator/member/canonical recorded", tracked)
	}
	if len(applied) != 1 {
		t.Fatalf("len(applied) = %d, want 1", len(applied))
	}
	if got := applied[0]; got.Name != 11 || !got.Fixed || string(got.KeyBytes) != "stored:derived" || got.KeyKind != runtime.VKString {
		t.Fatalf("applied[0] = %+v", got)
	}
}

func TestApplyDefaultsWithoutStoreAttrsSkipsKeyMaterialization(t *testing.T) {
	t.Parallel()

	uses := []runtime.AttrUse{{
		Name:      11,
		Validator: 21,
	}}
	selection := Selection{
		Present: true,
		Value:   runtime.ValueRef{Present: true, Off: 3, Len: 4},
	}

	tracked := 0
	materializeCalls := 0
	applied, err := ApplyDefaults(
		uses,
		nil,
		false,
		false,
		make([]Applied, 0, 8),
		func(*runtime.AttrUse) Selection { return selection },
		func(runtime.ValidatorID) bool { return false },
		func(runtime.ValueRef) []byte { return []byte("canon") },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID) error {
			tracked++
			return nil
		},
		func(runtime.ValidatorID, []byte, runtime.ValidatorID, runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
			materializeCalls++
			return runtime.VKString, []byte("derived"), nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ApplyDefaults() error = %v", err)
	}
	if tracked != 1 {
		t.Fatalf("tracked = %d, want 1", tracked)
	}
	if materializeCalls != 0 {
		t.Fatalf("materializeCalls = %d, want 0", materializeCalls)
	}
	if len(applied) != 1 {
		t.Fatalf("len(applied) = %d, want 1", len(applied))
	}
	if got := applied[0]; got.Name != 11 || got.KeyKind != runtime.VKInvalid || len(got.KeyBytes) != 0 {
		t.Fatalf("applied[0] = %+v", got)
	}
}

func TestApplyDefaultsPropagatesTrackError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	_, err := ApplyDefaults(
		[]runtime.AttrUse{{Validator: 1}},
		nil,
		false,
		false,
		nil,
		func(*runtime.AttrUse) Selection {
			return Selection{Present: true, Value: runtime.ValueRef{Present: true}}
		},
		nil,
		func(runtime.ValueRef) []byte { return []byte("canon") },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID) error { return wantErr },
		func(runtime.ValidatorID, []byte, runtime.ValidatorID, runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
			return runtime.VKInvalid, nil, nil
		},
		nil,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ApplyDefaults() error = %v, want %v", err, wantErr)
	}
}
