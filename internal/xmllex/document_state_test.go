package xmllex

import "testing"

func TestDocumentStateStartAndEnd(t *testing.T) {
	st := NewDocumentState()
	if st.RootSeen() {
		t.Fatalf("root seen before any token")
	}
	if st.RootClosed() {
		t.Fatalf("root closed before any token")
	}
	if !st.StartElementAllowed() {
		t.Fatalf("start element unexpectedly disallowed")
	}

	st.OnStartElement()
	if !st.RootSeen() {
		t.Fatalf("root not marked seen after start")
	}
	if st.RootClosed() {
		t.Fatalf("root unexpectedly closed after start")
	}

	st.OnEndElement(true)
	if !st.RootClosed() {
		t.Fatalf("root not marked closed after closeRoot end token")
	}
	if st.StartElementAllowed() {
		t.Fatalf("start element allowed after root closed")
	}
}

func TestDocumentStateOutsideCharDataBOM(t *testing.T) {
	st := NewDocumentState()
	if !st.ValidateOutsideCharData([]byte("\uFEFF")) {
		t.Fatalf("expected leading BOM to be allowed")
	}
	if st.ValidateOutsideCharData([]byte("\uFEFF")) {
		t.Fatalf("expected second BOM outside root to be rejected")
	}
}

func TestDocumentStateOutsideMarkupDisablesBOM(t *testing.T) {
	st := NewDocumentState()
	st.OnOutsideMarkup()
	if st.ValidateOutsideCharData([]byte("\uFEFF")) {
		t.Fatalf("BOM should be rejected after outside markup")
	}
}
