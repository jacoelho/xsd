package xsd_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

const (
	pathXSI       = ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	maxPathBytes  = 4096
	pathLaxSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:any processContents="lax" minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
)

func nestedDocument(names []string, leaf string) string {
	var doc strings.Builder
	doc.WriteString("<root" + pathXSI + ">")
	for _, name := range names {
		doc.WriteString("<" + name + ">")
	}
	doc.WriteString(leaf)
	for _, name := range slices.Backward(names) {
		doc.WriteString("</" + name + ">")
	}
	doc.WriteString("</root>")
	return doc.String()
}

func pathDiagnostics(t *testing.T, err error) []*xsderrors.Error {
	t.Helper()
	var out []*xsderrors.Error
	for _, leaf := range xsderrors.Flatten(err) {
		diagnostic, ok := errors.AsType[*xsderrors.Error](leaf)
		if !ok {
			t.Fatalf("diagnostic %T %v is not structured", leaf, leaf)
		}
		out = append(out, diagnostic)
	}
	return out
}

func distinctNames(count, size int) []string {
	names := make([]string, count)
	for i := range names {
		prefix := fmt.Sprintf("n%03d", i)
		names[i] = prefix + strings.Repeat("a", size-len(prefix))
	}
	return names
}

func TestInstanceDiagnosticPathsAreBoundedByMaxErrors(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	names := distinctNames(8, 256<<10)
	leaves := strings.Repeat(`<x xsi:type="xs:int">q</x>`, 100)
	err := engine.Validate(strings.NewReader(nestedDocument(names, leaves)))
	diagnostics := pathDiagnostics(t, err)
	if len(diagnostics) != 100 {
		t.Fatalf("diagnostics = %d, want 100", len(diagnostics))
	}
	total := 0
	for _, diagnostic := range diagnostics {
		if len(diagnostic.Path()) > maxPathBytes {
			t.Fatalf("path length = %d, want at most %d", len(diagnostic.Path()), maxPathBytes)
		}
		total += len(diagnostic.Path()) + len(diagnostic.Message())
	}
	if total > 100*8<<10 {
		t.Fatalf("retained path and message text = %d bytes, want at most %d", total, 100*8<<10)
	}
}

func TestInstanceDiagnosticPathEscapesNamespaceLineBreaks(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	doc := `<root` + pathXSI + ` xmlns:p="urn:a&#10;b"><p:x xsi:type="xs:int">q</p:x></root>`
	err := engine.Validate(strings.NewReader(doc))
	diagnostics := pathDiagnostics(t, err)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %v", err)
	}
	if got, want := diagnostics[0].Path(), `/root/{urn:a\nb}x`; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("Error() contains a raw line break: %q", err.Error())
	}
}

func TestInstanceDiagnosticPathElidesTheMiddle(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	names := distinctNames(200, 64)
	leaf := names[len(names)-1]
	err := engine.Validate(strings.NewReader(nestedDocument(names[:len(names)-1], `<`+leaf+` xsi:type="xs:int">q</`+leaf+`>`)))
	diagnostics := pathDiagnostics(t, err)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %v", err)
	}
	path := diagnostics[0].Path()
	if len(path) > maxPathBytes {
		t.Fatalf("path length = %d", len(path))
	}
	if strings.Count(path, "/...") != 1 {
		t.Fatalf("path has %d elisions, want 1", strings.Count(path, "/..."))
	}
	parts := strings.Split(path[1:], "/")
	if parts[0] != "root" || parts[1] != names[0] || parts[len(parts)-1] != leaf {
		t.Fatalf("path ends = %q ... %q, want root/%s ... %s", parts[:2], parts[len(parts)-1], names[0], leaf)
	}
	seenElision := false
	var kept []string
	for _, part := range parts[1:] {
		if part == "..." {
			seenElision = true
			continue
		}
		kept = append(kept, part)
	}
	if !seenElision {
		t.Fatal("elision missing")
	}
	// Each kept name is a whole input name, prefix names first and suffix names
	// last, in document order.
	last := -1
	for _, part := range kept {
		found := -1
		for i, name := range names {
			if name == part {
				found = i
			}
		}
		if found <= last {
			t.Fatalf("path segment %q out of order or not a whole input name", part)
		}
		last = found
	}
}

func TestInstanceDiagnosticPathBoundary(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	root := "root"
	segment := func(i, size int) string {
		prefix := fmt.Sprintf("m%02d", i)
		return prefix + strings.Repeat("c", size-len(prefix))
	}
	for _, test := range []struct {
		name   string
		elided bool
	}{{"exact", false}, {"one byte over", true}} {
		t.Run(test.name, func(t *testing.T) {
			inner := make([]string, 62)
			var want strings.Builder
			want.WriteString("/" + root)
			for i := range inner {
				inner[i] = segment(i, 63)
				want.WriteString("/" + inner[i])
			}
			// 5 ("/root") + 62*64 = 3973; a 123-byte leaf segment ("/" plus 122
			// bytes) brings the path to exactly 4096 bytes.
			leafLen := 122
			if test.elided {
				leafLen++
			}
			leaf := segment(99, leafLen)
			want.WriteString("/" + leaf)
			doc := nestedDocument(inner, `<`+leaf+` xsi:type="xs:int">q</`+leaf+`>`)
			diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
			if len(diagnostics) != 1 {
				t.Fatalf("diagnostics = %d, want 1", len(diagnostics))
			}
			path := diagnostics[0].Path()
			if !test.elided {
				if len(want.String()) != maxPathBytes || path != want.String() {
					t.Fatalf("path length = %d, want the full %d-byte path", len(path), len(want.String()))
				}
				return
			}
			if len(path) > maxPathBytes || !strings.Contains(path, "/...") {
				t.Fatalf("path length = %d, want an elided path of at most %d bytes", len(path), maxPathBytes)
			}
		})
	}
}

func TestInstanceDiagnosticPathWorstCaseExpansionIsBounded(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	var doc strings.Builder
	doc.WriteString(`<root` + pathXSI + ` xmlns:p="` + strings.Repeat("&#10;", 128) + `">`)
	for range 40 {
		doc.WriteString("<p:e>")
	}
	doc.WriteString(`<p:e xsi:type="xs:int">q</p:e>`)
	for range 40 {
		doc.WriteString("</p:e>")
	}
	doc.WriteString("</root>")
	diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc.String())))
	if len(diagnostics) != 1 || len(diagnostics[0].Path()) > maxPathBytes {
		t.Fatalf("diagnostics = %d, path length = %d", len(diagnostics), len(diagnostics[0].Path()))
	}
}

func TestInstanceDiagnosticPathsKeepShortExpandedNamesExact(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	doc := `<root` + pathXSI + ` xmlns:p="urn:x"><p:a xsi:type="xs:int">q</p:a></root>`
	diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
	if len(diagnostics) != 1 || diagnostics[0].Path() != "/root/{urn:x}a" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestIdentityDiagnosticPathsAreBounded(t *testing.T) {
	t.Run("duplicate under long names", func(t *testing.T) {
		engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath="*"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`)
		names := distinctNames(2, 256<<10)
		doc := `<root` + pathXSI + `>` +
			`<` + names[0] + ` xsi:type="xs:string">same</` + names[0] + `>` +
			`<` + names[1] + ` xsi:type="xs:string">same</` + names[1] + `></root>`
		diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
		if len(diagnostics) != 1 {
			t.Fatalf("diagnostics = %d, want 1", len(diagnostics))
		}
		if len(diagnostics[0].Path()) > maxPathBytes || len(diagnostics[0].Message()) > 2*maxPathBytes {
			t.Fatalf("path %d bytes, message %d bytes", len(diagnostics[0].Path()), len(diagnostics[0].Message()))
		}
	})
	t.Run("retained mixed lexical and expanded path", func(t *testing.T) {
		engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath=".//c"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`)
		doc := `<root` + pathXSI + ` xmlns:p="urn:a"><p:b><c xsi:type="xs:string">same</c></p:b><p:b><c xsi:type="xs:string">same</c></p:b></root>`
		diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
		if len(diagnostics) != 1 || diagnostics[0].Path() != "/root/{urn:a}b/c" ||
			!strings.Contains(diagnostics[0].Message(), "first seen at /root/{urn:a}b/c") {
			t.Fatalf("diagnostics = %v", diagnostics)
		}
	})
}

func TestMismatchedEndTagWithHugeNamesIsBounded(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	open, closing := strings.Repeat("a", 1<<20), strings.Repeat("b", 1<<20)
	err := engine.Validate(strings.NewReader("<root><" + open + "></" + closing + "></root>"))
	diagnostics := pathDiagnostics(t, err)
	if len(diagnostics) == 0 || diagnostics[0].Code() != xsderrors.CodeValidationXML || len(err.Error()) > 1<<10 {
		t.Fatalf("Validate() = %d bytes of %v, want a short validation.xml diagnostic", len(err.Error()), diagnostics)
	}
}

func TestPendingStartDiagnosticPaths(t *testing.T) {
	engine := compileLocationSchema(t, pathLaxSchema)
	t.Run("long ancestors", func(t *testing.T) {
		names := distinctNames(8, 256<<10)
		for _, test := range []struct {
			leaf string
			want int
		}{
			{`<x xsi:nil="bad"/>`, 1},
			{`<x xsi:nil="bad" xsi:schemaLocation="%zz"/>`, 2},
		} {
			doc := nestedDocument(names, test.leaf)
			diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
			if len(diagnostics) != test.want {
				t.Fatalf("%s: diagnostics = %d, want %d", test.leaf, len(diagnostics), test.want)
			}
			for _, diagnostic := range diagnostics {
				if len(diagnostic.Path()) > maxPathBytes {
					t.Fatalf("path length = %d", len(diagnostic.Path()))
				}
			}
		}
	})
	t.Run("exact path includes the pending child", func(t *testing.T) {
		doc := `<root` + pathXSI + ` xmlns:p="urn:p"><a><p:b xsi:nil="bad"/></a></root>`
		diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
		if len(diagnostics) != 1 || diagnostics[0].Path() != "/root/a/{urn:p}b" {
			t.Fatalf("diagnostics = %v", diagnostics)
		}
	})
}

func TestRejectedStartDoesNotLeakIntoLaterSiblingPaths(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="a" type="xs:int" minOccurs="0"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	err := engine.Validate(strings.NewReader("<root>\n<b/>\n<a>q</a>\n</root>"))
	var got []string
	for _, diagnostic := range pathDiagnostics(t, err) {
		got = append(got, fmt.Sprintf("%s %s", diagnostic.Code(), diagnostic.Path()))
	}
	want := []string{"validation.element /root", "validation.facet /root/a"}
	if strings.Join(got, ";") != strings.Join(want, ";") {
		t.Fatalf("diagnostics = %v, want %v", got, want)
	}
}
