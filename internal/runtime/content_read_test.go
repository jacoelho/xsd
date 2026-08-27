package runtime

import "testing"

func TestElementTextContentRead(t *testing.T) {
	t.Parallel()

	content := ElementTextContent{mixed: true, fixed: true, constrained: true}
	if !content.AllowsMixedContent() {
		t.Fatal("AllowsMixedContent() = false, want true")
	}
	if !content.HasFixedElementValue() {
		t.Fatal("HasFixedElementValue() = false, want true")
	}
	if !content.HasValueConstraint() {
		t.Fatal("HasValueConstraint() = false, want true")
	}

	var zero ElementTextContent
	if zero.AllowsMixedContent() || zero.HasFixedElementValue() || zero.HasValueConstraint() {
		t.Fatalf("zero ElementTextContent = %+v, want no flags", zero)
	}
}
