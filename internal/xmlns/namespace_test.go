package xmlns

import (
	"encoding/xml"
	"fmt"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
)

func TestStartRollsBackBindingsOnError(t *testing.T) {
	var ns Stack
	frame, element, err := ns.StartXML(xml.StartElement{Attr: []xml.Attr{
		{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: "a"}, Value: "urn:a"},
		{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: vocab.XMLPrefix}, Value: "urn:not-xml"},
	}})
	if err == nil {
		t.Fatal("StartXML() error = nil")
	}
	if !frame.IsZero() || element != (Element{}) {
		t.Fatalf("failed StartXML() returned frame/element = %+v/%+v", frame, element)
	}
	if ns.FrameCapacity() != 0 {
		t.Fatalf("FrameCapacity() = %d, want 0", ns.FrameCapacity())
	}
	if got, ok := ns.Lookup("a"); ok || got != "" {
		t.Fatalf("Lookup(a) = %q, %v; want missing", got, ok)
	}
}

func TestStartStreamResolvesNamesInPlace(t *testing.T) {
	var ns Stack
	start := stream.OwnedStartElement(xml.Name{Local: "root"},
		stream.OwnedAttr(xml.Name{Local: vocab.XMLNSPrefix}, "urn:default"),
		stream.OwnedAttr(xml.Name{Local: "attr"}, "value"),
	)
	frame, element, err := ns.StartStream(&start, nil)
	if err != nil {
		t.Fatalf("StartStream() error = %v", err)
	}
	if element.Name != (xml.Name{Space: "urn:default", Local: "root"}) {
		t.Fatalf("element name = %+v, want default namespace", element.Name)
	}
	if got := start.Attr[1].Name; got != (xml.Name{Local: "attr"}) {
		t.Fatalf("attribute name = %+v, want no namespace", got)
	}
	if err := ns.Abort(frame); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
}

func TestStackResetDropsOversizedCapacity(t *testing.T) {
	ns := NewStackWithCapacity(4, 4)
	ns.Reset(3)
	if ns.FrameCapacity() != 0 || ns.BindingCapacity() != 0 {
		t.Fatalf("Reset retained oversized capacities %d/%d", ns.FrameCapacity(), ns.BindingCapacity())
	}
}

func TestStackLifecycleZeroesReleasedStorage(t *testing.T) {
	var ns Stack
	frames := make([]Frame, 0, 32)
	for i := range 32 {
		prefix := fmt.Sprintf("p%d", i)
		frame, _, err := ns.StartXML(xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: prefix}, Value: "urn:" + prefix}}})
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	for _, frame := range slices.Backward(frames) {
		if err := ns.Abort(frame); err != nil {
			t.Fatal(err)
		}
	}
	for i, frame := range ns.frames[:cap(ns.frames)] {
		if frame != (stackFrame{}) {
			t.Fatalf("frame tail %d retains state: %+v", i, frame)
		}
	}
	for i, got := range ns.store.bindings[:cap(ns.store.bindings)] {
		if got != (binding{}) {
			t.Fatalf("binding tail %d retains references: %+v", i, got)
		}
	}

	frame, _, err := ns.StartXML(xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: "active"}, Value: "urn:active"}}})
	if err != nil {
		t.Fatal(err)
	}
	_ = frame
	ns.Reset(64)
	for i, got := range ns.store.bindings[:cap(ns.store.bindings)] {
		if got != (binding{}) {
			t.Fatalf("reset binding tail %d retains references: %+v", i, got)
		}
	}
}

func TestStartAdmissionIsAtomic(t *testing.T) {
	t.Parallel()
	var ns Stack
	failed := stream.OwnedStartElement(xml.Name{Space: "p", Local: "root"},
		stream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:p"),
		stream.OwnedAttr(xml.Name{Space: "p", Local: "id"}, "1"),
		stream.OwnedAttr(xml.Name{Space: "p", Local: "id"}, "2"),
	)
	if _, _, err := ns.StartStream(&failed, nil); err == nil {
		t.Fatal("StartStream() accepted duplicate expanded attributes")
	}
	if _, ok := ns.Lookup("p"); ok {
		t.Fatal("failed StartStream() retained a namespace binding")
	}

	start := stream.OwnedStartElement(xml.Name{Space: "p", Local: "root"},
		stream.OwnedAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:p"),
		stream.OwnedAttr(xml.Name{Space: "p", Local: "id"}, "1"),
	)
	_, element, err := ns.StartStream(&start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if element.Name != (xml.Name{Space: "urn:p", Local: "root"}) || start.Attr[1].Name != (xml.Name{Space: "urn:p", Local: "id"}) {
		t.Fatalf("StartStream() element/attribute = %+v/%+v", element, start.Attr[1].Name)
	}
}

func TestEndFailureLeavesOwnedFrameIntact(t *testing.T) {
	t.Parallel()
	var ns Stack
	frame, _, err := ns.StartXML(xml.StartElement{Name: xml.Name{Local: "root"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ns.End(frame, LexicalName{Local: "other"}); err == nil {
		t.Fatal("End() accepted a mismatched name")
	}
	if err := ns.End(frame, LexicalName{Local: "root"}); err != nil {
		t.Fatalf("End() after mismatch: %v", err)
	}
}

func TestFrameCannotCloseAnotherStackOrNonTopFrame(t *testing.T) {
	t.Parallel()
	var first, second Stack
	parent, _, err := first.StartXML(xml.StartElement{Name: xml.Name{Local: "parent"}})
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := first.StartXML(xml.StartElement{Name: xml.Name{Local: "child"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Abort(parent); err == nil {
		t.Fatal("Abort() accepted a non-top frame")
	}
	if err := second.Abort(child); err == nil {
		t.Fatal("Abort() accepted another stack's frame")
	}
}

func TestContextIsImmutableAcrossStackChangesAndReset(t *testing.T) {
	var ns Stack
	frame, _, err := ns.StartXML(xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, Value: "urn:first"}}})
	if err != nil {
		t.Fatal(err)
	}
	context := ns.Context()
	if err := ns.Abort(frame); err != nil {
		t.Fatal(err)
	}
	ns.Reset(64)
	if got, ok := context.Lookup("p"); !ok || got != "urn:first" {
		t.Fatalf("Context.Lookup(p) = %q, %v", got, ok)
	}
	if got := testing.AllocsPerRun(100, func() {
		view := ns.Context()
		_, _ = view.Lookup("")
	}); got != 0 {
		t.Fatalf("Context allocations = %g, want 0", got)
	}
}
