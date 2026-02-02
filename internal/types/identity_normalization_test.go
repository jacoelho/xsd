package types

import "testing"

func TestIdentityNormalizationCycleCachesResult(t *testing.T) {
	st := &SimpleType{
		QName: QName{Local: "A"},
		Union: &UnionType{},
	}
	st.MemberTypes = []Type{st}

	if IdentityNormalizable(st) {
		t.Fatalf("expected IdentityNormalizable to return false for cycle")
	}
	if !st.identityNormalizationReady {
		t.Fatalf("expected identityNormalizationReady to be true after computation")
	}
	if st.identityNormalizationComputing {
		t.Fatalf("expected identityNormalizationComputing to be false after computation")
	}

	if IdentityNormalizable(st) {
		t.Fatalf("expected IdentityNormalizable to remain false for cycle")
	}
	if !st.identityNormalizationReady || st.identityNormalizationComputing {
		t.Fatalf("expected identity normalization to remain ready and not computing")
	}
}
