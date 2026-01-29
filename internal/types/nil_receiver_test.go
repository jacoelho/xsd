package types

import "testing"

func TestSimpleTypeNilReceiver(t *testing.T) {
	var st *SimpleType

	if got := st.Name(); !got.IsZero() {
		t.Fatalf("Name() = %v, want zero QName", got)
	}
	if got := st.ComponentName(); !got.IsZero() {
		t.Fatalf("ComponentName() = %v, want zero QName", got)
	}
	if got := st.DeclaredNamespace(); got != "" {
		t.Fatalf("DeclaredNamespace() = %q, want empty", got)
	}
	if st.Copy(CopyOptions{}) != nil {
		t.Fatal("Copy() returned non-nil for nil receiver")
	}
	if st.IsBuiltin() {
		t.Fatal("IsBuiltin() returned true for nil receiver")
	}
	if st.BaseType() != nil {
		t.Fatal("BaseType() returned non-nil for nil receiver")
	}
	if st.ResolvedBaseType() != nil {
		t.Fatal("ResolvedBaseType() returned non-nil for nil receiver")
	}
	if st.WhiteSpace() != WhiteSpacePreserve {
		t.Fatalf("WhiteSpace() = %v, want %v", st.WhiteSpace(), WhiteSpacePreserve)
	}
	st.SetWhiteSpace(WhiteSpaceCollapse)
	st.SetWhiteSpaceExplicit(WhiteSpaceCollapse)
	if st.WhiteSpaceExplicit() {
		t.Fatal("WhiteSpaceExplicit() returned true for nil receiver")
	}
	if got := st.MeasureLength("a b"); got != 0 {
		t.Fatalf("MeasureLength() = %d, want 0", got)
	}
	if err := st.Validate("x"); err == nil {
		t.Fatal("Validate() returned nil error for nil receiver")
	}
	if _, err := st.ParseValue("x"); err == nil {
		t.Fatal("ParseValue() returned nil error for nil receiver")
	}
	if st.PrimitiveType() != nil {
		t.Fatal("PrimitiveType() returned non-nil for nil receiver")
	}
	if st.FundamentalFacets() != nil {
		t.Fatal("FundamentalFacets() returned non-nil for nil receiver")
	}
	if st.IsQNameOrNotationType() {
		t.Fatal("IsQNameOrNotationType() returned true for nil receiver")
	}
	if st.Variety() != AtomicVariety {
		t.Fatalf("Variety() = %v, want %v", st.Variety(), AtomicVariety)
	}
}
