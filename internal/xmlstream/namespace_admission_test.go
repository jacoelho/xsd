package xmlstream

import (
	"encoding/xml"
	"strconv"
	"testing"
)

func TestNamespaceAdmissionAcrossAttributeFreeChildren(t *testing.T) {
	var ns stack
	var values cache
	root := StartElement{
		Name: xml.Name{Local: "root"},
		Attr: []Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: "urn:default"},
			{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:prefix"},
		},
	}
	for i := range 20 {
		root.Attr = append(root.Attr, Attr{Name: xml.Name{Space: "p", Local: "a" + strconv.Itoa(i)}, Value: "root"})
	}
	rootFrame, _, err := ns.StartStream(&root, &values)
	if err != nil {
		t.Fatal(err)
	}
	context := ns.Context()
	for range 8 {
		child := StartElement{Name: xml.Name{Space: "p", Local: "child"}}
		childFrame, element, childErr := ns.StartStream(&child, &values)
		if childErr != nil {
			t.Fatal(childErr)
		}
		if element != (xml.Name{Space: "urn:prefix", Local: "child"}) {
			t.Fatalf("attribute-free child name = %+v", element)
		}
		if endErr := ns.End(childFrame, Lexical(child.Name)); endErr != nil {
			t.Fatal(endErr)
		}
	}
	for _, width := range []int{16, 17, 0, 1, 17} {
		sibling := StartElement{Name: xml.Name{Local: "sibling"}}
		for i := range width {
			sibling.Attr = append(sibling.Attr, Attr{Name: xml.Name{Space: "p", Local: "a" + strconv.Itoa(i)}, Value: "sibling"})
		}
		siblingFrame, element, err := ns.StartStream(&sibling, &values)
		if err != nil {
			t.Fatalf("reusing %d expanded names: %v", width, err)
		}
		if element != (xml.Name{Space: "urn:default", Local: "sibling"}) {
			t.Fatalf("sibling name = %+v", element)
		}
		for i, attr := range sibling.Attr {
			if attr.Name != (xml.Name{Space: "urn:prefix", Local: "a" + strconv.Itoa(i)}) || attr.Value != "sibling" {
				t.Fatalf("published sibling attribute %d = %+v", i, attr)
			}
		}
		if endErr := ns.End(siblingFrame, Lexical(sibling.Name)); endErr != nil {
			t.Fatal(endErr)
		}
	}
	if endErr := ns.End(rootFrame, Lexical(root.Name)); endErr != nil {
		t.Fatal(endErr)
	}
	ns.Reset(64)
	for prefix, want := range map[string]string{"": "urn:default", "p": "urn:prefix"} {
		if got, ok := context.Lookup(prefix); !ok || got != want {
			t.Fatalf("retained context %q = %q/%v, want %q", prefix, got, ok, want)
		}
	}
}

func TestNamespaceAliasFailureThenAttributeFreeAndAttributedSiblings(t *testing.T) {
	var ns stack
	var values cache
	root := StartElement{Name: xml.Name{Local: "root"}, Attr: []Attr{
		{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:parent"},
		{Name: xml.Name{Space: "xmlns", Local: "q"}, Value: "urn:parent"},
		{Name: xml.Name{Space: "p", Local: "value"}, Value: "root"},
	}}
	rootFrame, _, err := ns.StartStream(&root, &values)
	if err != nil {
		t.Fatal(err)
	}
	context := ns.Context()
	wide := StartElement{Name: xml.Name{Space: "p", Local: "wide"}}
	for i := range 17 {
		wide.Attr = append(wide.Attr, Attr{Name: xml.Name{Space: "p", Local: "a" + strconv.Itoa(i)}, Value: "wide"})
	}
	wideFrame, _, err := ns.StartStream(&wide, &values)
	if err != nil {
		t.Fatal(err)
	}
	if err := ns.End(wideFrame, Lexical(wide.Name)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		label, wantError string
		tail             []Attr
	}{
		{"aliased duplicate", "duplicate attribute {urn:shadow}dup", []Attr{
			{Name: xml.Name{Space: "p", Local: "dup"}, Value: "first"},
			{Name: xml.Name{Space: "q", Local: "dup"}, Value: "second"},
		}},
		{"unbound attribute", "unbound namespace prefix missing", []Attr{
			{Name: xml.Name{Space: "missing", Local: "value"}, Value: "last"},
		}},
	} {
		t.Run(test.label, func(t *testing.T) {
			failed := StartElement{Name: xml.Name{Space: "p", Local: "failed"}, Attr: []Attr{
				{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:shadow"},
				{Name: xml.Name{Space: "xmlns", Local: "q"}, Value: "urn:shadow"},
			}}
			for i := range 17 {
				failed.Attr = append(failed.Attr, Attr{Name: xml.Name{Space: "p", Local: "a" + strconv.Itoa(i)}, Value: "partial"})
			}
			failed.Attr = append(failed.Attr, test.tail...)
			var wantNames []xml.Name
			var wantValues []string
			for _, attr := range failed.Attr {
				wantNames = append(wantNames, attr.Name)
				wantValues = append(wantValues, attr.Value)
			}
			if _, _, err := ns.StartStream(&failed, &values); err == nil || err.Error() != test.wantError {
				t.Fatalf("admission error = %v, want %q", err, test.wantError)
			}
			for i, attr := range failed.Attr {
				if attr.Name != wantNames[i] || attr.Value != wantValues[i] {
					t.Fatalf("failed admission changed lexical attribute %d to %+v", i, attr)
				}
			}
			if ns.depth() != 1 {
				t.Fatalf("depth after failure = %d, want root only", ns.depth())
			}
			for _, prefix := range []string{"p", "q"} {
				if got, ok := ns.Lookup(prefix); !ok || got != "urn:parent" {
					t.Fatalf("parent binding %s = %q/%v", prefix, got, ok)
				}
				if got, ok := context.Lookup(prefix); !ok || got != "urn:parent" {
					t.Fatalf("retained binding %s = %q/%v", prefix, got, ok)
				}
			}
			for _, width := range []int{0, 1, 17} {
				child := StartElement{Name: xml.Name{Space: "p", Local: "child"}}
				for i := range width {
					child.Attr = append(child.Attr, Attr{Name: xml.Name{Space: "p", Local: "a" + strconv.Itoa(i)}, Value: "retry"})
				}
				childFrame, element, err := ns.StartStream(&child, &values)
				if err != nil || element != (xml.Name{Space: "urn:parent", Local: "child"}) {
					t.Fatalf("admission after failure = %+v / %v", element, err)
				}
				for i, attr := range child.Attr {
					if attr.Name != (xml.Name{Space: "urn:parent", Local: "a" + strconv.Itoa(i)}) || attr.Value != "retry" {
						t.Fatalf("retry attribute %d = %+v", i, attr)
					}
				}
				if err := ns.End(childFrame, Lexical(child.Name)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if err := ns.End(rootFrame, Lexical(root.Name)); err != nil {
		t.Fatal(err)
	}
	ns.Reset(64)
	for _, prefix := range []string{"p", "q"} {
		if got, ok := context.Lookup(prefix); !ok || got != "urn:parent" {
			t.Fatalf("retained binding after Reset %s = %q/%v", prefix, got, ok)
		}
	}
}

func TestNamespaceAdmissionRetentionAfterEmptyStarts(t *testing.T) {
	for _, limit := range []int{16, 17, 18} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			var ns stack
			var values cache
			for _, width := range []int{17, 0, 1} {
				start := StartElement{Name: xml.Name{Local: "e"}}
				for i := range width {
					start.Attr = append(start.Attr, Attr{Name: xml.Name{Local: "a" + strconv.Itoa(i)}, Value: "before"})
				}
				admitted, _, err := ns.StartStream(&start, &values)
				if err != nil {
					t.Fatal(err)
				}
				if err := ns.End(admitted, Lexical(start.Name)); err != nil {
					t.Fatal(err)
				}
			}
			beforeCap := cap(ns.resolvedAttrs)
			ns.Reset(limit)
			if beforeCap > limit && cap(ns.resolvedAttrs) != 0 {
				t.Fatalf("oversized resolved capacity %d retained at limit%d", beforeCap, limit)
			}
			if cap(ns.resolvedAttrs) > limit {
				t.Fatalf("retained resolved capacity%d exceeds limit%d", cap(ns.resolvedAttrs), limit)
			}
			if limit < 17 && ns.seen.index != nil {
				t.Fatal("wide duplicate map retained below its admission peak")
			}
			start := StartElement{Name: xml.Name{Local: "e"}}
			for i := range 17 {
				start.Attr = append(start.Attr, Attr{Name: xml.Name{Local: "a" + strconv.Itoa(i)}, Value: "after"})
			}
			reuseFrame, element, err := ns.StartStream(&start, &values)
			if err != nil || element != (xml.Name{Local: "e"}) {
				t.Fatalf("valid admission after retention reset = %+v / %v", element, err)
			}
			for i, attr := range start.Attr {
				if attr.Name != (xml.Name{Local: "a" + strconv.Itoa(i)}) || attr.Value != "after" {
					t.Fatalf("published attribute after Reset %d = %+v", i, attr)
				}
			}
			if err := ns.End(reuseFrame, Lexical(start.Name)); err != nil {
				t.Fatal(err)
			}
			duplicate := StartElement{Name: xml.Name{Local: "e"}, Attr: []Attr{
				{Name: xml.Name{Local: "a0"}, Value: "first"},
				{Name: xml.Name{Local: "a0"}, Value: "second"},
			}}
			if _, _, err := ns.StartStream(&duplicate, &values); err == nil || err.Error() != "duplicate attribute a0" {
				t.Fatalf("duplicate after Reset = %v", err)
			}
		})
	}
}
