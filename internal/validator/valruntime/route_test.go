package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalRoute(t *testing.T) {
	tests := []struct {
		kind runtime.ValidatorKind
		want Route
	}{
		{kind: runtime.VString, want: RouteAtomic},
		{kind: runtime.VDateTime, want: RouteTemporal},
		{kind: runtime.VAnyURI, want: RouteAnyURI},
		{kind: runtime.VQName, want: RouteQName},
		{kind: runtime.VHexBinary, want: RouteHexBinary},
		{kind: runtime.VBase64Binary, want: RouteBase64Binary},
		{kind: runtime.VList, want: RouteList},
		{kind: runtime.VUnion, want: RouteUnion},
		{kind: runtime.ValidatorKind(255), want: RouteInvalid},
	}

	for _, tc := range tests {
		if got := CanonicalRoute(tc.kind); got != tc.want {
			t.Fatalf("CanonicalRoute(%v) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestNoCanonicalRoute(t *testing.T) {
	tests := []struct {
		kind runtime.ValidatorKind
		want Route
	}{
		{kind: runtime.VString, want: RouteAtomic},
		{kind: runtime.VDateTime, want: RouteTemporal},
		{kind: runtime.VAnyURI, want: RouteAnyURI},
		{kind: runtime.VHexBinary, want: RouteHexBinary},
		{kind: runtime.VBase64Binary, want: RouteBase64Binary},
		{kind: runtime.VList, want: RouteList},
		{kind: runtime.VQName, want: RouteInvalid},
		{kind: runtime.VUnion, want: RouteInvalid},
	}

	for _, tc := range tests {
		if got := NoCanonicalRoute(tc.kind); got != tc.want {
			t.Fatalf("NoCanonicalRoute(%v) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}
