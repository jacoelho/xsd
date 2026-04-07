package validator

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalizeListRuntimeBuildsCanonicalValueAndKey(t *testing.T) {
	out, bufs, err := canonicalizeListRuntime(
		runtime.ValidatorMeta{
			Kind:       runtime.VList,
			Index:      0,
			WhiteSpace: runtime.WSCollapse,
		},
		runtime.ValidatorsBundle{
			List: []runtime.ListValidator{{Item: 7}},
		},
		[]byte("a b"),
		true,
		true,
		listBuffers{
			Value: make([]byte, 0, 8),
			Key:   make([]byte, 0, 16),
		},
		func(itemValidator runtime.ValidatorID, item []byte, needKey bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			if itemValidator != 7 {
				t.Fatalf("item validator = %d, want 7", itemValidator)
			}
			canon := append([]byte(nil), item...)
			for i, b := range canon {
				if b >= 'a' && b <= 'z' {
					canon[i] = b - ('a' - 'A')
				}
			}
			return canon, runtime.VKString, append([]byte("k:"), item...), needKey, nil
		},
	)
	if err != nil {
		t.Fatalf("canonicalizeListRuntime() error = %v", err)
	}
	if got := string(out.Canonical); got != "A B" {
		t.Fatalf("canonicalizeListRuntime() canonical = %q, want %q", got, "A B")
	}
	if out.Count != 2 {
		t.Fatalf("canonicalizeListRuntime() count = %d, want 2", out.Count)
	}
	if !out.KeySet || len(out.Key) == 0 {
		t.Fatal("canonicalizeListRuntime() should derive a list key")
	}
	if len(bufs.Value) == 0 || len(bufs.Key) == 0 {
		t.Fatal("canonicalizeListRuntime() should reuse caller-owned buffers")
	}
}

func TestCanonicalizeListRuntimeRequiresItemKeyWhenRequested(t *testing.T) {
	_, _, err := canonicalizeListRuntime(
		runtime.ValidatorMeta{
			Kind:       runtime.VList,
			Index:      0,
			WhiteSpace: runtime.WSCollapse,
		},
		runtime.ValidatorsBundle{List: []runtime.ListValidator{{Item: 1}}},
		[]byte("a"),
		true,
		true,
		listBuffers{},
		func(runtime.ValidatorID, []byte, bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			return []byte("a"), runtime.VKInvalid, nil, false, nil
		},
	)
	if err == nil || err.Error() != "list item key missing" {
		t.Fatalf("canonicalizeListRuntime() error = %v, want list item key missing", err)
	}
}

func TestValidateListNoCanonicalRuntimeUsesCollapsedFloatFastPath(t *testing.T) {
	called := false
	err := validateListNoCanonicalRuntime(
		runtime.ValidatorMeta{
			Kind:       runtime.VList,
			Index:      0,
			WhiteSpace: runtime.WSCollapse,
		},
		runtime.ValidatorsBundle{
			List: []runtime.ListValidator{{Item: 2}},
			Meta: []runtime.ValidatorMeta{
				{},
				{},
				{Kind: runtime.VDouble},
			},
		},
		[]byte("1 2.5 -3"),
		true,
		func(runtime.ValidatorID, []byte) error {
			called = true
			return errors.New("unexpected callback")
		},
	)
	if err != nil {
		t.Fatalf("validateListNoCanonicalRuntime() error = %v", err)
	}
	if called {
		t.Fatal("validateListNoCanonicalRuntime() should use the collapsed-float fast path")
	}
}

func TestValidateListNoCanonicalRuntimeVisitsItemsWithoutFastPath(t *testing.T) {
	count := 0
	err := validateListNoCanonicalRuntime(
		runtime.ValidatorMeta{
			Kind:       runtime.VList,
			Index:      0,
			WhiteSpace: runtime.WSPreserve,
		},
		runtime.ValidatorsBundle{
			List: []runtime.ListValidator{{Item: 3}},
			Meta: []runtime.ValidatorMeta{
				{},
				{},
				{},
				{Kind: runtime.VString},
			},
		},
		[]byte("a\tb"),
		false,
		func(itemValidator runtime.ValidatorID, item []byte) error {
			if itemValidator != 3 {
				t.Fatalf("item validator = %d, want 3", itemValidator)
			}
			count++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("validateListNoCanonicalRuntime() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("validateListNoCanonicalRuntime() item count = %d, want 2", count)
	}
}

func TestValidateCollapsedFloatListRuntime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "finite", input: "1 .25 -0.5 1.5E2", want: true},
		{name: "signed finite", input: "+1 -.25 -1.0e+2", want: true},
		{name: "special literals", input: "INF -INF NaN", want: true},
		{name: "plus inf invalid", input: "+INF", want: false},
		{name: "dangling sign", input: "-", want: false},
		{name: "bad exponent", input: "1e", want: false},
		{name: "bad literal terminator", input: "INFx", want: false},
		{name: "bad numeric terminator", input: "1x", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCollapsedFloatListRuntime([]byte(tt.input), runtime.VDouble)
			if (err == nil) != tt.want {
				t.Fatalf("validateCollapsedFloatListRuntime(%q) err = %v, want success=%v", tt.input, err, tt.want)
			}
		})
	}
}
