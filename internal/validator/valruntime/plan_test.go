package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		name           string
		meta           runtime.ValidatorMeta
		opts           Options
		hasLengthFacet bool
		want           Plan
	}{
		{
			name: "enum forces key and local metrics",
			meta: runtime.ValidatorMeta{
				Kind:  runtime.VString,
				Flags: runtime.ValidatorHasEnum,
			},
			opts: Options{ApplyWhitespace: true},
			want: Plan{
				NeedKey:          true,
				NeedLocalMetrics: true,
			},
		},
		{
			name: "binary length facet keeps local metrics and clone",
			meta: runtime.ValidatorMeta{
				Kind: runtime.VBase64Binary,
			},
			opts:           Options{RequireCanonical: true},
			hasLengthFacet: true,
			want: Plan{
				NeedCanonical:    true,
				NeedLocalMetrics: true,
				CloneCanonical:   true,
			},
		},
		{
			name: "union tracking ids needs local metrics and scratch normalization",
			meta: runtime.ValidatorMeta{
				Kind:  runtime.VUnion,
				Flags: runtime.ValidatorMayTrackIDs,
			},
			opts: Options{
				ApplyWhitespace: true,
				TrackIDs:        true,
			},
			want: Plan{
				NeedCanonical:           true,
				NeedLocalMetrics:        true,
				UseScratchNormalization: true,
			},
		},
		{
			name: "store value forces canonical and key",
			meta: runtime.ValidatorMeta{
				Kind: runtime.VString,
			},
			opts: Options{
				StoreValue: true,
			},
			want: Plan{
				NeedCanonical: true,
				NeedKey:       true,
			},
		},
		{
			name: "notation forces canonical",
			meta: runtime.ValidatorMeta{
				Kind: runtime.VNotation,
			},
			opts: Options{},
			want: Plan{
				NeedCanonical: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Build(tc.meta, tc.opts, tc.hasLengthFacet); got != tc.want {
				t.Fatalf("Build() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestMemberLookupOptions(t *testing.T) {
	got := MemberLookupOptions()
	want := Options{
		ApplyWhitespace:  true,
		RequireCanonical: true,
	}
	if got != want {
		t.Fatalf("MemberLookupOptions() = %+v, want %+v", got, want)
	}
}
