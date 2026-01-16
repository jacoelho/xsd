package xmlstream

import "testing"

func TestEventKindString(t *testing.T) {
	//nolint:govet // keep table fields readable.
	tests := []struct {
		kind EventKind
		want string
	}{
		{EventStartElement, "StartElement"},
		{EventEndElement, "EndElement"},
		{EventCharData, "CharData"},
		{EventComment, "Comment"},
		{EventPI, "PI"},
		{EventDirective, "Directive"},
		{EventKind(99), "EventKind(99)"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Fatalf("EventKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestQNameMethods(t *testing.T) {
	q := QName{Namespace: "urn:test", Local: "item"}
	if !q.Is("urn:test", "item") {
		t.Fatalf("QName.Is true = false")
	}
	if q.Is("urn:other", "item") {
		t.Fatalf("QName.Is other namespace = true, want false")
	}
	if q.Is("urn:test", "other") {
		t.Fatalf("QName.Is other local = true, want false")
	}
	if !q.HasLocal("item") {
		t.Fatalf("QName.HasLocal true = false")
	}
	if got := q.String(); got != "{urn:test}item" {
		t.Fatalf("QName.String = %q, want {urn:test}item", got)
	}
	if got := (QName{Local: "plain"}).String(); got != "plain" {
		t.Fatalf("QName.String no namespace = %q, want plain", got)
	}
}

func TestEventAttrLocalFirstMatch(t *testing.T) {
	ev := Event{
		Attrs: []Attr{
			{Name: QName{Namespace: "urn:a", Local: "id"}, Value: []byte("first")},
			{Name: QName{Namespace: "urn:b", Local: "id"}, Value: []byte("second")},
		},
	}
	if got, ok := ev.AttrLocal("id"); !ok || string(got) != "first" {
		t.Fatalf("AttrLocal = %q, ok=%v, want first, true", string(got), ok)
	}
}

func TestRawNameHasLocal(t *testing.T) {
	name := RawName{Local: []byte("item")}
	if !name.HasLocal([]byte("item")) {
		t.Fatalf("RawName.HasLocal = false, want true")
	}
	if name.HasLocal([]byte("other")) {
		t.Fatalf("RawName.HasLocal other = true, want false")
	}
}

func TestRawNameHasLocalEmpty(t *testing.T) {
	name := RawName{Local: nil}
	if !name.HasLocal([]byte{}) {
		t.Fatalf("RawName.HasLocal empty = false, want true")
	}
	if name.HasLocal([]byte("x")) {
		t.Fatalf("RawName.HasLocal non-empty = true, want false")
	}
}

func TestEventAttrLookup(t *testing.T) {
	ev := Event{
		Attrs: []Attr{
			{Name: QName{Namespace: "", Local: "id"}, Value: []byte("1")},
			{Name: QName{Namespace: "urn:x", Local: "id"}, Value: []byte("2")},
			{Name: QName{Namespace: "urn:x", Local: "lang"}, Value: []byte("en")},
		},
	}
	if got, ok := ev.Attr("", "id"); !ok || string(got) != "1" {
		t.Fatalf("Attr id = %q, ok=%v, want 1, true", string(got), ok)
	}
	if got, ok := ev.Attr("urn:x", "lang"); !ok || string(got) != "en" {
		t.Fatalf("Attr lang = %q, ok=%v, want en, true", string(got), ok)
	}
	if _, ok := ev.Attr("urn:missing", "id"); ok {
		t.Fatalf("Attr missing namespace = true, want false")
	}
	if got, ok := ev.AttrLocal("id"); !ok || string(got) != "1" {
		t.Fatalf("AttrLocal id = %q, ok=%v, want 1, true", string(got), ok)
	}
	if _, ok := ev.AttrLocal("missing"); ok {
		t.Fatalf("AttrLocal missing = true, want false")
	}
}

func TestEventAttrEmptySlice(t *testing.T) {
	ev := Event{Attrs: nil}
	if _, ok := ev.Attr("", "x"); ok {
		t.Fatalf("Attr empty = true, want false")
	}
	if _, ok := ev.AttrLocal("x"); ok {
		t.Fatalf("AttrLocal empty = true, want false")
	}
}

func TestRawEventAttrLookup(t *testing.T) {
	ev := RawEvent{
		Attrs: []RawAttr{
			{Name: RawName{Prefix: []byte("x"), Local: []byte("id")}, Value: []byte("1")},
			{Name: RawName{Prefix: []byte("y"), Local: []byte("id")}, Value: []byte("2")},
			{Name: RawName{Prefix: []byte("x"), Local: []byte("lang")}, Value: []byte("en")},
		},
	}
	if got, ok := ev.Attr([]byte("x"), []byte("id")); !ok || string(got) != "1" {
		t.Fatalf("Attr x:id = %q, ok=%v, want 1, true", string(got), ok)
	}
	if got, ok := ev.Attr([]byte("x"), []byte("lang")); !ok || string(got) != "en" {
		t.Fatalf("Attr x:lang = %q, ok=%v, want en, true", string(got), ok)
	}
	if _, ok := ev.Attr([]byte("z"), []byte("id")); ok {
		t.Fatalf("Attr missing prefix = true, want false")
	}
	if got, ok := ev.AttrLocal([]byte("id")); !ok || string(got) != "1" {
		t.Fatalf("AttrLocal id = %q, ok=%v, want 1, true", string(got), ok)
	}
	if _, ok := ev.AttrLocal([]byte("missing")); ok {
		t.Fatalf("AttrLocal missing = true, want false")
	}
}

func TestRawEventAttrUnprefixed(t *testing.T) {
	ev := RawEvent{
		Attrs: []RawAttr{
			{Name: RawName{Local: []byte("id")}, Value: []byte("1")},
		},
	}
	if got, ok := ev.Attr(nil, []byte("id")); !ok || string(got) != "1" {
		t.Fatalf("Attr nil prefix = %q, ok=%v, want 1, true", string(got), ok)
	}
	if got, ok := ev.Attr([]byte(""), []byte("id")); !ok || string(got) != "1" {
		t.Fatalf("Attr empty prefix = %q, ok=%v, want 1, true", string(got), ok)
	}
}

func TestRawEventAttrEmptySlice(t *testing.T) {
	ev := RawEvent{Attrs: nil}
	if _, ok := ev.Attr(nil, []byte("x")); ok {
		t.Fatalf("Attr empty = true, want false")
	}
	if _, ok := ev.AttrLocal([]byte("x")); ok {
		t.Fatalf("AttrLocal empty = true, want false")
	}
}

func TestRawEventAttrDuplicateLocal(t *testing.T) {
	ev := RawEvent{
		Attrs: []RawAttr{
			{Name: RawName{Prefix: []byte("x"), Local: []byte("id")}, Value: []byte("1")},
			{Name: RawName{Prefix: []byte("y"), Local: []byte("id")}, Value: []byte("2")},
		},
	}
	if got, ok := ev.Attr([]byte("y"), []byte("id")); !ok || string(got) != "2" {
		t.Fatalf("Attr y:id = %q, ok=%v, want 2, true", string(got), ok)
	}
	if got, ok := ev.AttrLocal([]byte("id")); !ok || string(got) != "1" {
		t.Fatalf("AttrLocal id = %q, ok=%v, want 1, true", string(got), ok)
	}
}
