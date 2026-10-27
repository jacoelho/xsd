package value

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestPatternWorkLimitStopsBeforeAlternatives(t *testing.T) {
	pattern, err := xsdregex.Compile(`(a|aa)*b`, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	alternative, err := xsdregex.Compile(`aab`, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	groups := [][]*Pattern{{pattern, alternative}}
	for _, raw := range []bool{false, true} {
		var scratch Scratch
		for _, work := range []uint64{26, 27, 26} {
			scratch.PatternOptions = xsdregex.MatchOptions{MaxWork: work, MaxStates: 4}
			var err error
			if raw {
				err = applyPatternFacetsBytes(groups, []byte("aab"), &scratch)
			} else {
				err = applyPatternFacets(groups, "aab", &scratch)
			}
			if work == 26 {
				if !errors.Is(err, ErrLimit) {
					t.Fatalf("bytes=%v, work=%d: %v, want ErrLimit before the alternative", raw, work, err)
				}
			} else if err != nil {
				t.Fatalf("bytes=%v, work=%d: %v", raw, work, err)
			}
		}
	}
}
