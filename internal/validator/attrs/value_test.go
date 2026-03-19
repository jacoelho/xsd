package attrs

import (
	"errors"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func TestValidateValueTracksDuplicateID(t *testing.T) {
	t.Parallel()

	seenID := true
	_, err := ValidateValue(
		nil,
		Start{Value: []byte("id")},
		false,
		ValueSpec{Validator: 7},
		&seenID,
		ValidateValueCallbacks{
			Validate: func(runtime.ValidatorID, []byte, bool) (ValueResult, error) {
				return ValueResult{}, nil
			},
			IsIDValidator: func(runtime.ValidatorID) bool { return true },
		},
	)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrMultipleIDAttr {
		t.Fatalf("ValidateValue() error = %v, want %s", err, xsderrors.ErrMultipleIDAttr)
	}
}

func TestValidateValueAppendsCanonicalAndMatchesFixed(t *testing.T) {
	t.Parallel()

	var appended []Start
	seenID := false
	validated, err := ValidateValue(
		nil,
		Start{Value: []byte("lexical")},
		true,
		ValueSpec{
			Validator: 5,
			Fixed:     runtime.ValueRef{Present: true},
		},
		&seenID,
		ValidateValueCallbacks{
			Validate: func(id runtime.ValidatorID, lexical []byte, store bool) (ValueResult, error) {
				if id != 5 || string(lexical) != "lexical" || !store {
					t.Fatalf("Validate() got id=%d lexical=%q store=%v", id, lexical, store)
				}
				return ValueResult{
					Canonical: []byte("canon"),
					KeyKind:   runtime.VKString,
					KeyBytes:  []byte("key"),
					HasKey:    true,
				}, nil
			},
			AppendCanonical: func(validated []Start, attr Start, store bool, canonical []byte, keyKind runtime.ValueKind, keyBytes []byte) []Start {
				appended = append(appended, Start{Value: canonical, KeyKind: keyKind, KeyBytes: keyBytes})
				return append(validated, attr)
			},
			MatchFixed: func(spec ValueSpec, result ValueResult) (bool, error) {
				if !spec.Fixed.Present || string(result.Canonical) != "canon" || !result.HasKey {
					t.Fatalf("MatchFixed() got spec=%+v result=%+v", spec, result)
				}
				return true, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateValue() error = %v", err)
	}
	if seenID {
		t.Fatal("seenID = true, want false")
	}
	if len(validated) != 1 || len(appended) != 1 || string(appended[0].Value) != "canon" || string(appended[0].KeyBytes) != "key" {
		t.Fatalf("validated = %#v appended = %#v", validated, appended)
	}
}

func TestValidateValuePropagatesValidateError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	_, err := ValidateValue(
		nil,
		Start{Value: []byte("lexical")},
		false,
		ValueSpec{Validator: 3},
		new(bool),
		ValidateValueCallbacks{
			Validate: func(runtime.ValidatorID, []byte, bool) (ValueResult, error) {
				return ValueResult{}, wantErr
			},
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ValidateValue() error = %v, want %v", err, wantErr)
	}
}

func TestValidateValueRejectsFixedMismatch(t *testing.T) {
	t.Parallel()

	_, err := ValidateValue(
		nil,
		Start{Value: []byte("lexical")},
		false,
		ValueSpec{
			Validator: 9,
			Fixed:     runtime.ValueRef{Present: true},
		},
		new(bool),
		ValidateValueCallbacks{
			Validate: func(runtime.ValidatorID, []byte, bool) (ValueResult, error) {
				return ValueResult{}, nil
			},
			MatchFixed: func(ValueSpec, ValueResult) (bool, error) {
				return false, nil
			},
		},
	)
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrAttributeFixedValue {
		t.Fatalf("ValidateValue() error = %v, want %s", err, xsderrors.ErrAttributeFixedValue)
	}
}
