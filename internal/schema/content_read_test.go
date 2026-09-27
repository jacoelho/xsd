package schema

import "testing"

func TestElementTextContentRead(t *testing.T) {
	t.Parallel()

	content := ElementTextContent{kind: ContentMixed, fixed: true, constrained: true}
	if !content.AllowsMixedContent() {
		t.Fatal("AllowsMixedContent() = false, want true")
	}
	if !content.HasFixedElementValue() {
		t.Fatal("HasFixedElementValue() = false, want true")
	}
	if !content.HasValueConstraint() {
		t.Fatal("HasValueConstraint() = false, want true")
	}
	if content.IsEmptyContent() {
		t.Fatal("IsEmptyContent() = true, want false for mixed content")
	}

	var zero ElementTextContent
	if zero.AllowsMixedContent() || zero.HasFixedElementValue() || zero.HasValueConstraint() {
		t.Fatalf("zero ElementTextContent = %+v, want no flags", zero)
	}
	empty := ElementTextContent{kind: ContentEmpty}
	if !empty.IsEmptyContent() || empty.AllowsMixedContent() {
		t.Fatalf("empty ElementTextContent = %+v, want empty and non-mixed", empty)
	}
}
