package attrs

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSelectDefaultOrFixed(t *testing.T) {
	defaultRef := runtime.ValueRef{Off: 1, Len: 2, Present: true}
	fixedRef := runtime.ValueRef{Off: 3, Len: 4, Present: true}
	defaultKey := runtime.ValueKeyRef{Kind: runtime.VKString, Ref: runtime.ValueRef{Off: 7, Len: 3, Present: true}}
	fixedKey := runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: runtime.ValueRef{Off: 10, Len: 5, Present: true}}

	tests := []struct {
		name string
		use  *runtime.AttrUse
		want Selection
	}{
		{
			name: "nil use",
			want: Selection{},
		},
		{
			name: "empty use",
			use:  &runtime.AttrUse{},
			want: Selection{},
		},
		{
			name: "default only",
			use: &runtime.AttrUse{
				Default:       defaultRef,
				DefaultKey:    defaultKey,
				DefaultMember: 9,
			},
			want: Selection{
				Value:   defaultRef,
				Key:     defaultKey,
				Member:  9,
				Present: true,
			},
		},
		{
			name: "fixed wins over default",
			use: &runtime.AttrUse{
				Default:       defaultRef,
				DefaultKey:    defaultKey,
				DefaultMember: 9,
				Fixed:         fixedRef,
				FixedKey:      fixedKey,
				FixedMember:   11,
			},
			want: Selection{
				Value:   fixedRef,
				Key:     fixedKey,
				Member:  11,
				Fixed:   true,
				Present: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelectDefaultOrFixed(tc.use); got != tc.want {
				t.Fatalf("SelectDefaultOrFixed() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
