package xsd_test

import (
	"errors"
	"io"
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

// TestUPAInsideReferencedGroupNamesTheLowestEnclosingModel pins the
// precision limit of content-model location: particles carry no source, so a
// violation inside a group is reported by the first model, in compilation
// order, whose check reaches it. Types compile before groups, so a referring
// type in another document can be named; an unreferenced group names itself.
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
		{
			name:    "unreferenced",
			sources: []xsd.SchemaSource{xsd.Bytes("b.xsd", []byte(group))},
			want:    compileLocation{path: "b.xsd", line: 3},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.Compile(test.sources...)
			if !strings.Contains(err.Error(), "UPA violation: overlapping particles \"a\"") {
				t.Fatalf("error = %v, want the automaton's UPA violation on a", err)
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
