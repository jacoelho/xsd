package xsd_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type compileLocation struct {
	path string
	line int
}

func compileDiagnosticLocation(t *testing.T, err error, code xsderrors.Code) compileLocation {
	t.Helper()
	leaves := xsderrors.Flatten(err)
	if len(leaves) != 1 {
		t.Fatalf("diagnostics = %v, want one", leaves)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](leaves[0])
	if !ok || diagnostic.Code() != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	return compileLocation{path: diagnostic.Path(), line: diagnostic.Line()}
}

func TestSourceSetFailuresCarryNoLocation(t *testing.T) {
	t.Parallel()

	two := []xsd.SchemaSource{
		xsd.Bytes("a.xsd", []byte(`<xs:schema `+xsNS+`/>`)),
		xsd.Bytes("b.xsd", []byte(`<xs:schema `+xsNS+`/>`)),
	}
	for _, test := range []struct {
		name string
		opts xsd.CompileOptions
		srcs []xsd.SchemaSource
		code xsderrors.Code
	}{
		{"no sources", xsd.CompileOptions{}, nil, xsderrors.CodeSchemaNoSources},
		{"more sources than MaxSchemaSources", xsd.CompileOptions{MaxSchemaSources: 1}, two, xsderrors.CodeSchemaLimit},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.CompileWithOptions(test.opts, test.srcs...)
			if got := compileDiagnosticLocation(t, err, test.code); got != (compileLocation{}) {
				t.Fatalf("location = %+v, want none: %v", got, err)
			}
		})
	}
}

func TestWholeDocumentFailuresCarryOnlyThePath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		opts xsd.CompileOptions
		path string
		src  xsd.SchemaSource
		code xsderrors.Code
	}{
		{
			name: "explicit source that cannot be opened",
			path: "missing.xsd",
			src:  xsd.Open("missing.xsd", func() (io.ReadCloser, error) { return nil, errors.New("no such file") }),
			code: xsderrors.CodeSchemaRead,
		},
		{
			name: "explicit source above MaxSchemaSourceBytes",
			opts: xsd.CompileOptions{MaxSchemaSourceBytes: 8},
			path: "big.xsd",
			src:  xsd.Bytes("big.xsd", []byte(`<xs:schema `+xsNS+`/>`)),
			code: xsderrors.CodeSchemaLimit,
		},
		{
			name: "explicit source reaching MaxSchemaSourceBytes in the prolog",
			opts: xsd.CompileOptions{MaxSchemaSourceBytes: 23},
			path: "prolog.xsd",
			src:  xsd.Bytes("prolog.xsd", []byte("<?xml version=\"1.0\"?>\n\n<xs:schema "+xsNS+"/>")),
			code: xsderrors.CodeSchemaLimit,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.CompileWithOptions(test.opts, test.src)
			want := compileLocation{path: test.path}
			if got := compileDiagnosticLocation(t, err, test.code); got != want {
				t.Fatalf("location = %+v, want %+v: %v", got, want, err)
			}
		})
	}
}

func TestIncompleteSchemaDocumentIsLocatedWhereReadingStopped(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, text string
		code       xsderrors.Code
		line       int
	}{
		{"empty", "", xsderrors.CodeSchemaRoot, 1},
		{"comment only", "<!-- c -->\n\n", xsderrors.CodeSchemaRoot, 3},
		{"unclosed root", "<xs:schema " + xsNS + ">\n<xs:element name=\"e\"/>\n", xsderrors.CodeSchemaXML, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.Compile(xsd.Bytes("doc.xsd", []byte(test.text)))
			want := compileLocation{path: "doc.xsd", line: test.line}
			if got := compileDiagnosticLocation(t, err, test.code); got != want {
				t.Fatalf("location = %+v, want %+v: %v", got, want, err)
			}
		})
	}
}

func TestSchemaReaderFailuresAreLocatedLikeInstanceReaderFailures(t *testing.T) {
	t.Parallel()

	instanceEngine, err := xsd.Compile(xsd.Bytes("instance.xsd", []byte(`<xs:schema `+xsNS+`><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile(instance.xsd) error = %v", err)
	}
	root := `<xs:schema ` + xsNS + `/>`
	for _, test := range []struct {
		name, text   string
		code         xsderrors.Code
		line, column int
	}{
		{"empty document", "", xsderrors.CodeSchemaRoot, 1, 1},
		{"prolog ending at a line start", "<?pi?>\n", xsderrors.CodeSchemaRoot, 2, 1},
		{"text outside root starting with a line break", root + "\nx", xsderrors.CodeSchemaXML, 1, len(root) + 1},
		{"unclosed root at end of input", "<xs:schema " + xsNS + ">\n", xsderrors.CodeSchemaXML, 2, 1},
		{"unclosed child at end of input", "<xs:schema " + xsNS + ">\n<xs:annotation>\n", xsderrors.CodeSchemaXML, 3, 1},
		{"declared ISO-8859-1 encoding", `<?xml version="1.0" encoding="ISO-8859-1"?>` + root, xsderrors.CodeUnsupportedNonUTF8, 1, 1},
		{"UTF-16 big-endian byte order mark", "\xFE\xFF\x00<", xsderrors.CodeUnsupportedNonUTF8, 1, 1},
		{"UTF-16 little-endian byte order mark", "\xFF\xFE<\x00", xsderrors.CodeUnsupportedNonUTF8, 1, 1},
		{"XML version 1.1", `<?xml version="1.1"?>` + root, xsderrors.CodeUnsupportedXMLVersion, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, compileErr := xsd.Compile(xsd.Bytes("doc.xsd", []byte(test.text)))
			schemaDiagnostic := singleDiagnostic(t, compileErr)
			if schemaDiagnostic.Code() != test.code {
				t.Fatalf("Compile() code = %s, want %s: %v", schemaDiagnostic.Code(), test.code, compileErr)
			}
			if schemaDiagnostic.Line() != test.line || schemaDiagnostic.Column() != test.column {
				t.Fatalf("Compile() location = %d:%d, want %d:%d: %v", schemaDiagnostic.Line(), schemaDiagnostic.Column(), test.line, test.column, compileErr)
			}
			validateErr := instanceEngine.Validate(strings.NewReader(test.text))
			instanceDiagnostic := singleDiagnostic(t, validateErr)
			if instanceDiagnostic.Line() != test.line || instanceDiagnostic.Column() != test.column {
				t.Fatalf("Validate() location = %d:%d, want %d:%d: %v", instanceDiagnostic.Line(), instanceDiagnostic.Column(), test.line, test.column, validateErr)
			}
		})
	}
}

func singleDiagnostic(t *testing.T, err error) *xsderrors.Error {
	t.Helper()
	leaves := xsderrors.Flatten(err)
	if len(leaves) != 1 {
		t.Fatalf("diagnostics = %v, want one", leaves)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](leaves[0])
	if !ok {
		t.Fatalf("diagnostic = %T, want *xsderrors.Error", leaves[0])
	}
	return diagnostic
}

// TestUPAInsideReferencedGroupNamesTheLowestEnclosingModel pins the
// precision limit of content-model location: particles carry no source, so a
// violation inside a group is reported by the first model, in compilation
// order, whose check reaches it. Types compile before groups, so a referring
// type in another document can be named.
func TestUPAInsideReferencedGroupNamesTheLowestEnclosingModel(t *testing.T) {
	t.Parallel()

	// The repeated choice needs a runtime split, so the violation is found
	// while constructing the automaton, not by the direct particle check.
	group := strings.Join([]string{
		`<xs:schema ` + xsNS + `>`,
		`<xs:group name="G">`,
		`<xs:sequence>`,
		`<xs:choice maxOccurs="2"><xs:element name="a" maxOccurs="2"/><xs:element name="b"/></xs:choice>`,
		`<xs:element name="a" minOccurs="0"/>`,
		`</xs:sequence>`,
		`</xs:group>`,
		`</xs:schema>`,
	}, "\n")
	referrer := strings.Join([]string{
		`<xs:schema ` + xsNS + `>`,
		`<xs:complexType name="T">`,
		`<xs:sequence><xs:group ref="G"/></xs:sequence>`,
		`</xs:complexType>`,
		`</xs:schema>`,
	}, "\n")
	for _, test := range []struct {
		name    string
		sources []xsd.SchemaSource
		want    compileLocation
	}{
		{
			name:    "referenced from another document",
			sources: []xsd.SchemaSource{xsd.Bytes("a.xsd", []byte(referrer)), xsd.Bytes("b.xsd", []byte(group))},
			want:    compileLocation{path: "a.xsd", line: 3},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.Compile(test.sources...)
			if !strings.Contains(err.Error(), `UPA violation in complex type "T": element "a" and element "a" overlap`) {
				t.Fatalf("error = %v, want the automaton's UPA violation on a in T", err)
			}
			if got := compileDiagnosticLocation(t, err, xsderrors.CodeSchemaContentModel); got != test.want {
				t.Fatalf("location = %+v, want %+v: %v", got, test.want, err)
			}
		})
	}
}

func TestUPAInsideGroupIsLocatedAtTheFirstModelInSourceOrderThatReachesIt(t *testing.T) {
	t.Parallel()

	ambiguousGroup := `<xs:choice><xs:element name="a"/><xs:sequence><xs:element name="a"/></xs:sequence></xs:choice>`
	ambiguousWhenRepeated := `<xs:sequence><xs:element name="a"/><xs:element name="a" minOccurs="0"/></xs:sequence>`
	referrers := map[string]func(maxOccurs string) []string{
		"nested": func(maxOccurs string) []string {
			return []string{
				`<xs:complexType name="T"><xs:sequence><xs:element name="x"/><xs:group ref="G" maxOccurs="` + maxOccurs + `"/></xs:sequence></xs:complexType>`,
			}
		},
		"extension": func(maxOccurs string) []string {
			return []string{
				`<xs:complexType name="B"><xs:sequence><xs:element name="z"/></xs:sequence></xs:complexType>`,
				`<xs:complexType name="T"><xs:complexContent><xs:extension base="B"><xs:group ref="G" maxOccurs="` + maxOccurs + `"/></xs:extension></xs:complexContent></xs:complexType>`,
			}
		},
		"whole content": func(maxOccurs string) []string {
			return []string{
				`<xs:complexType name="T"><xs:group ref="G" maxOccurs="` + maxOccurs + `"/></xs:complexType>`,
			}
		},
		"whole content beside a unit reference": func(maxOccurs string) []string {
			return []string{
				`<xs:complexType name="T"><xs:group ref="G" maxOccurs="` + maxOccurs + `"/></xs:complexType>`,
				`<xs:complexType name="U"><xs:sequence><xs:group ref="G"/></xs:sequence></xs:complexType>`,
			}
		},
	}
	atGroup := compileLocation{path: "b.xsd", line: 3}
	for _, test := range []struct {
		shape     string
		group     string
		maxOccurs string
		want      compileLocation
	}{
		{shape: "nested", group: ambiguousGroup, maxOccurs: "1", want: atGroup},
		{shape: "nested", group: ambiguousGroup, maxOccurs: "2", want: atGroup},
		{shape: "nested", group: ambiguousGroup, maxOccurs: "unbounded", want: atGroup},
		{shape: "extension", group: ambiguousGroup, maxOccurs: "1", want: atGroup},
		{shape: "extension", group: ambiguousGroup, maxOccurs: "2", want: atGroup},
		{shape: "extension", group: ambiguousGroup, maxOccurs: "unbounded", want: atGroup},
		{shape: "whole content", group: ambiguousGroup, maxOccurs: "1", want: atGroup},
		{shape: "whole content", group: ambiguousGroup, maxOccurs: "2", want: compileLocation{path: "a.xsd", line: 2}},
		{shape: "whole content", group: ambiguousGroup, maxOccurs: "unbounded", want: compileLocation{path: "a.xsd", line: 2}},
		{shape: "whole content beside a unit reference", group: ambiguousGroup, maxOccurs: "2", want: atGroup},
		{shape: "nested", group: ambiguousWhenRepeated, maxOccurs: "2", want: compileLocation{path: "a.xsd", line: 2}},
		{shape: "extension", group: ambiguousWhenRepeated, maxOccurs: "2", want: compileLocation{path: "a.xsd", line: 3}},
		{shape: "whole content", group: ambiguousWhenRepeated, maxOccurs: "2", want: compileLocation{path: "a.xsd", line: 2}},
	} {
		name := test.shape + " maxOccurs " + test.maxOccurs
		if test.group == ambiguousWhenRepeated {
			name += " ambiguous only when repeated"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			referrer := strings.Join(slices.Concat(
				[]string{`<xs:schema ` + xsNS + `>`},
				referrers[test.shape](test.maxOccurs),
				[]string{`</xs:schema>`},
			), "\n")
			group := strings.Join([]string{
				`<xs:schema ` + xsNS + `>`,
				`<xs:group name="G">`,
				test.group,
				`</xs:group>`,
				`</xs:schema>`,
			}, "\n")
			_, err := xsd.Compile(xsd.Bytes("a.xsd", []byte(referrer)), xsd.Bytes("b.xsd", []byte(group)))
			if err == nil || !strings.Contains(err.Error(), "UPA violation") {
				t.Fatalf("error = %v, want a UPA violation", err)
			}
			if got := compileDiagnosticLocation(t, err, xsderrors.CodeSchemaContentModel); got != test.want {
				t.Fatalf("location = %+v, want %+v: %v", got, test.want, err)
			}
		})
	}
}

// TestContentModelBudgetFailuresNameTheCheckedComponent sweeps
// MaxContentModelAnalysisSteps over two explicit roots. The first root holds no
// content model, so a failure inside a checked component names the second
// document; only set-wide work names the first root.
func TestContentModelBudgetFailuresNameTheCheckedComponent(t *testing.T) {
	t.Parallel()

	first := `<xs:schema ` + xsNS + `><xs:simpleType name="s"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`
	second := strings.Join([]string{
		`<xs:schema ` + xsNS + `>`,
		`<xs:complexType name="base"><xs:sequence><xs:element name="a" maxOccurs="3"/><xs:element name="b" minOccurs="0"/></xs:sequence></xs:complexType>`,
		`<xs:complexType name="derived"><xs:complexContent><xs:restriction base="base">`,
		`<xs:sequence><xs:element name="a" maxOccurs="2"/></xs:sequence>`,
		`</xs:restriction></xs:complexContent></xs:complexType>`,
		`</xs:schema>`,
	}, "\n")
	const derivedLine = 3
	sources := []xsd.SchemaSource{xsd.Bytes("a.xsd", []byte(first)), xsd.Bytes("b.xsd", []byte(second))}
	atDerived := false
	for limit := 1; limit <= 100_000; limit++ {
		engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxContentModelAnalysisSteps: limit}, sources...)
		if err == nil {
			if engine == nil {
				t.Fatal("successful compilation returned no engine")
			}
			if !atDerived {
				t.Fatalf("no budget failure named the derived type at b.xsd:%d", derivedLine)
			}
			return
		}
		got := compileDiagnosticLocation(t, err, xsderrors.CodeSchemaLimit)
		if got.path == "" || got.line <= 0 {
			t.Fatalf("limit %d: failure is unlocated: %v", limit, err)
		}
		if got == (compileLocation{path: "b.xsd", line: derivedLine}) {
			atDerived = true
		}
	}
	t.Fatal("schema did not compile within the swept content-model budgets")
}
