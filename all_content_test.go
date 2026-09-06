package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func allContentSchema(width int, declarations, particles string) string {
	var doc strings.Builder
	doc.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	doc.WriteString(declarations)
	doc.WriteString(`<xs:element name="root"><xs:complexType><xs:all>`)
	doc.WriteString(particles)
	for i := range width {
		fmt.Fprintf(&doc, `<xs:element name="e%d" type="xs:string" minOccurs="0"/>`, i)
	}
	doc.WriteString(`</xs:all></xs:complexType></xs:element></xs:schema>`)
	return doc.String()
}

func allContentDocument(width int, middle string) string {
	var doc strings.Builder
	doc.WriteString(`<root>`)
	for i := width - 1; i >= 0; i-- {
		fmt.Fprintf(&doc, `<e%d/>`, i)
	}
	doc.WriteString(middle)
	doc.WriteString(`</root>`)
	return doc.String()
}

func TestAllContentDispatch(t *testing.T) {
	t.Parallel()
	for _, width := range []int{4, 16, 65, 256} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(xsd.Bytes("all.xsd", []byte(allContentSchema(width, `<xs:element name="head" type="xs:string" abstract="true"/><xs:element name="member" type="xs:string" substitutionGroup="head"/>`, `<xs:element ref="head"/>`))))
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name, xml string
				valid     bool
			}{
				{"reverse with substitution", allContentDocument(width, `<member/>`), true},
				{"optional absent", `<root><member/></root>`, true},
				{"required absent", allContentDocument(width, ""), false},
				{"abstract head", allContentDocument(width, `<head/>`), false},
				{"repeated element", allContentDocument(width, `<e0/><member/>`), false},
				{"repeated substitution", allContentDocument(width, `<member/><member/>`), false},
				{"unknown child", allContentDocument(width, `<other/><member/>`), false},
				{"reuse after errors", allContentDocument(width, `<member/>`), true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					err := session.Validate(strings.NewReader(tc.xml))
					if (err == nil) != tc.valid {
						t.Fatalf("Validate() = %v, valid = %v", err, tc.valid)
					}
				})
			}
		})
	}
}

func BenchmarkSessionValidateAllContent(b *testing.B) {
	for _, width := range []int{4, 16, 64, 256, 1024} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("all.xsd", []byte(allContentSchema(width, "", ""))))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			doc := allContentDocument(width, "")
			var reader strings.Reader
			reader.Reset(doc)
			if err := session.Validate(&reader); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(doc)))
			for b.Loop() {
				reader.Reset(doc)
				if err := session.Validate(&reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
