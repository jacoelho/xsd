package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestGValueIdentityMatchesValueSpaceEquality(t *testing.T) {
	t.Parallel()

	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		typ         string
		left, right string
		equal       bool
	}{
		{name: "gYearMonth zero offset", typ: "gYearMonth", left: "2024-02Z", right: "2024-02+00:00", equal: true},
		{name: "gYearMonth different instant", typ: "gYearMonth", left: "2024-02Z", right: "2024-02+01:00"},
		{name: "gYear zero offset", typ: "gYear", left: "2024Z", right: "2024+00:00", equal: true},
		{name: "gYear different instant", typ: "gYear", left: "2024Z", right: "2024+01:00"},
		{name: "gMonthDay cross-day", typ: "gMonthDay", left: "--01-02+14:00", right: "--01-01-10:00", equal: true},
		{name: "gMonthDay different instant", typ: "gMonthDay", left: "--01-01Z", right: "--01-01+01:00"},
		{name: "gDay cross-day", typ: "gDay", left: "---02+14:00", right: "---01-10:00", equal: true},
		{name: "gDay different instant", typ: "gDay", left: "---01Z", right: "---01+01:00"},
		{name: "gMonth zero offset", typ: "gMonth", left: "--02Z", right: "--02+00:00", equal: true},
		{name: "gMonth different instant", typ: "gMonth", left: "--02Z", right: "--02+01:00"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			id, ok := value.BuiltinTypeID(test.typ)
			if !ok {
				t.Fatalf("missing builtin %q", test.typ)
			}
			left, err := program.Validate(id, test.left, value.Resolver{}, value.NeedIdentity, nil)
			if err != nil {
				t.Fatalf("validate left %q: %v", test.left, err)
			}
			right, err := program.Validate(id, test.right, value.Resolver{}, value.NeedIdentity, nil)
			if err != nil {
				t.Fatalf("validate right %q: %v", test.right, err)
			}
			if got := left.Equal(right); got != test.equal {
				t.Fatalf("Equal(%q, %q) = %v, want %v", test.left, test.right, got, test.equal)
			}
			if got := left.IdentityKey() == right.IdentityKey(); got != test.equal {
				t.Fatalf("identity key equality for %q and %q = %v, want %v (%q / %q)", test.left, test.right, got, test.equal, left.IdentityKey(), right.IdentityKey())
			}
		})
	}
}

func TestGValueIdentityDistinguishesTimezonePresence(t *testing.T) {
	t.Parallel()

	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		typ, without, with string
	}{
		{typ: "gYearMonth", without: "2024-02", with: "2024-02Z"},
		{typ: "gYear", without: "2024", with: "2024Z"},
		{typ: "gMonthDay", without: "--01-01", with: "--01-01Z"},
		{typ: "gDay", without: "---01", with: "---01Z"},
		{typ: "gMonth", without: "--02", with: "--02Z"},
	} {
		t.Run(test.typ, func(t *testing.T) {
			t.Parallel()
			id, ok := value.BuiltinTypeID(test.typ)
			if !ok {
				t.Fatalf("missing builtin %q", test.typ)
			}
			without, err := program.Validate(id, test.without, value.Resolver{}, value.NeedIdentity, nil)
			if err != nil {
				t.Fatalf("validate no-timezone value %q: %v", test.without, err)
			}
			with, err := program.Validate(id, test.with, value.Resolver{}, value.NeedIdentity, nil)
			if err != nil {
				t.Fatalf("validate timezone value %q: %v", test.with, err)
			}
			if without.Equal(with) {
				t.Fatalf("Equal(%q, %q) = true, want false", test.without, test.with)
			}
			if without.IdentityKey() == with.IdentityKey() {
				t.Fatalf("identity keys for %q and %q are equal", test.without, test.with)
			}
		})
	}
}
