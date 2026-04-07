package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestBuildValueExecutionPlan(t *testing.T) {
	tests := []struct {
		name           string
		meta           runtime.ValidatorMeta
		opts           valueOptions
		hasLengthFacet bool
		want           valueExecutionPlan
	}{
		{
			name: "enum forces key and local metrics",
			meta: runtime.ValidatorMeta{
				Kind:  runtime.VString,
				Flags: runtime.ValidatorHasEnum,
			},
			opts: valueOptions{ApplyWhitespace: true},
			want: valueExecutionPlan{
				NeedKey:          true,
				NeedLocalMetrics: true,
			},
		},
		{
			name: "binary length facet keeps local metrics and clone",
			meta: runtime.ValidatorMeta{
				Kind: runtime.VBase64Binary,
			},
			opts:           valueOptions{RequireCanonical: true},
			hasLengthFacet: true,
			want: valueExecutionPlan{
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
			opts: valueOptions{
				ApplyWhitespace: true,
				TrackIDs:        true,
			},
			want: valueExecutionPlan{
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
			opts: valueOptions{
				StoreValue: true,
			},
			want: valueExecutionPlan{
				NeedCanonical: true,
				NeedKey:       true,
			},
		},
		{
			name: "notation forces canonical",
			meta: runtime.ValidatorMeta{
				Kind: runtime.VNotation,
			},
			opts: valueOptions{},
			want: valueExecutionPlan{
				NeedCanonical: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildValueExecutionPlan(tc.meta, tc.opts, tc.hasLengthFacet); got != tc.want {
				t.Fatalf("buildValueExecutionPlan() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
