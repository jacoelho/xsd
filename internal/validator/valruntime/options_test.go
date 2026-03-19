package valruntime

import "testing"

func TestAttributeOptions(t *testing.T) {
	got := AttributeOptions(true, true)
	want := Options{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: true,
		StoreValue:       true,
		NeedKey:          true,
	}
	if got != want {
		t.Fatalf("AttributeOptions() = %+v, want %+v", got, want)
	}
}

func TestTextOptions(t *testing.T) {
	got := TextOptions(true, false, true)
	want := Options{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: true,
		StoreValue:       true,
	}
	if got != want {
		t.Fatalf("TextOptions() = %+v, want %+v", got, want)
	}
}

func TestListItemOptions(t *testing.T) {
	parent := Options{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: false,
		StoreValue:       true,
	}
	got := ListItemOptions(parent, true)
	want := Options{
		RequireCanonical: true,
		NeedKey:          true,
	}
	if got != want {
		t.Fatalf("ListItemOptions() = %+v, want %+v", got, want)
	}
}

func TestListNoCanonicalItemOptions(t *testing.T) {
	parent := Options{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: true,
		StoreValue:       true,
		NeedKey:          true,
	}
	got := ListNoCanonicalItemOptions(parent)
	if got != (Options{}) {
		t.Fatalf("ListNoCanonicalItemOptions() = %+v, want zero options", got)
	}
}

func TestUnionMemberOptions(t *testing.T) {
	parent := Options{
		ApplyWhitespace: false,
		TrackIDs:        true,
		StoreValue:      true,
	}
	got := UnionMemberOptions(parent, true, true)
	want := Options{
		ApplyWhitespace:  true,
		RequireCanonical: true,
		NeedKey:          true,
	}
	if got != want {
		t.Fatalf("UnionMemberOptions() = %+v, want %+v", got, want)
	}
}

func TestTextNeedsMetrics(t *testing.T) {
	if TextNeedsMetrics(Options{}) {
		t.Fatal("TextNeedsMetrics(empty) = true, want false")
	}
	if !TextNeedsMetrics(Options{StoreValue: true}) {
		t.Fatal("TextNeedsMetrics(store-value) = false, want true")
	}
	if !TextNeedsMetrics(Options{NeedKey: true}) {
		t.Fatal("TextNeedsMetrics(need-key) = false, want true")
	}
}
