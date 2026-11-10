package xsd_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestNilResolverFuncReturnsSchemaMiss(t *testing.T) {
	t.Parallel()

	var resolver xsd.ResolverFunc
	_, err := resolver.ResolveSchema("root.xsd", "child.xsd")
	if !errors.Is(err, xsderrors.ErrSchemaNotFound) {
		t.Fatalf("ResolveSchema() error = %v, want schema miss", err)
	}
}

func TestNilResolversDoNotSplitPublicSourceGraph(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:child"/></xs:schema>`
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.xsd")
	if err := os.WriteFile(rootPath, []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}

	var nilResolver xsd.ResolverFunc
	var nilResolverPointer *xsd.ResolverFunc
	memory := xsd.Bytes("memory-root.xsd", []byte(schema))
	tests := []struct {
		name    string
		sources []xsd.SchemaSource
	}{
		{
			name:    "file explicit nil resolver",
			sources: []xsd.SchemaSource{xsd.File(rootPath), xsd.File(rootPath).WithResolver(nil)},
		},
		{
			name:    "typed nil resolver function",
			sources: []xsd.SchemaSource{memory, memory.WithResolver(nilResolver)},
		},
		{
			name:    "typed nil resolver function pointer",
			sources: []xsd.SchemaSource{memory, memory.WithResolver(nilResolverPointer)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaReferences: 1}, test.sources...); err != nil {
				t.Fatalf("CompileWithOptions() = %v, want equivalent nil resolver contexts to deduplicate", err)
			}
		})
	}
}

func TestCompileSourceLimitKeepsSourcePathForJoinedAcquisitionErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		readErr  error
		closeErr error
	}{
		{name: "read", readErr: errors.New("read failed at limit")},
		{name: "close", closeErr: errors.New("close failed at limit")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.CompileWithOptions(
				xsd.CompileOptions{MaxSchemaSourceBytes: 1},
				xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
					return &sourceRegressionReadCloser{
						Reader:   strings.NewReader("ab"),
						readErr:  test.readErr,
						closeErr: test.closeErr,
					}, nil
				}),
			)
			if err == nil {
				t.Fatal("CompileWithOptions() succeeded beyond source limit")
			}
			wantCause := test.readErr
			if wantCause == nil {
				wantCause = test.closeErr
			}
			if !errors.Is(err, wantCause) {
				t.Fatalf("CompileWithOptions() error = %v, want cause %v", err, wantCause)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit || diagnostic.Path() != "schema.xsd" {
				t.Fatalf("CompileWithOptions() diagnostic = %v, want schema limit at schema.xsd", err)
			}
			flat := xsderrors.Flatten(err)
			if len(flat) < 2 {
				t.Fatalf("Flatten(CompileWithOptions()) = %v, want limit plus acquisition cause", flat)
			}
			limit, ok := flat[0].(*xsderrors.Error) //nolint:errorlint // Flatten must expose the direct located diagnostic.
			if !ok || limit.Path() != "schema.xsd" {
				t.Fatalf("Flatten()[0] = %T %v, want located schema limit", flat[0], flat[0])
			}
		})
	}
}

// A schema.limit diagnostic returned by a caller's reader is the caller's
// read failure, not this compilation's own limit.
func TestCallerReaderSchemaLimitErrorIsSchemaRead(t *testing.T) {
	t.Parallel()

	_, inner := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSourceBytes: 1}, xsd.Bytes("inner.xsd", []byte("ab")))
	expectSourceCode(t, inner, xsderrors.CodeSchemaLimit)
	_, err := xsd.Compile(xsd.Open("o.xsd", func() (io.ReadCloser, error) {
		return &sourceRegressionReadCloser{
			Reader:  strings.NewReader(`<xs:schema ` + sourceLedgerNS + `>`),
			readErr: inner,
		}, nil
	}))
	diagnostic := sourceDiagnostic(t, err, xsderrors.CodeSchemaRead)
	if diagnostic.Path() != "o.xsd" {
		t.Fatalf("Path() = %q, want o.xsd: %v", diagnostic.Path(), err)
	}
	if !errors.Is(err, inner) {
		t.Fatalf("error = %v, want cause %v", err, inner)
	}
}

type sourceRegressionReadCloser struct {
	*strings.Reader

	readErr  error
	closeErr error
}

func (r *sourceRegressionReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 && r.readErr != nil {
		return n, r.readErr
	}
	return n, err
}

func (r *sourceRegressionReadCloser) Close() error { return r.closeErr }

const sourceLedgerNS = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`

func ledgerSchema(targetNamespace string, imports ...string) string {
	var b strings.Builder
	b.WriteString(`<xs:schema ` + sourceLedgerNS + ` targetNamespace="` + targetNamespace + `">`)
	for _, location := range imports {
		b.WriteString(`<xs:import namespace="urn:` + location + `" schemaLocation="` + location + `.xsd"/>`)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

func sourceDiagnostic(t *testing.T, err error, code xsderrors.Code) *xsderrors.Error {
	t.Helper()
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || x.Code() != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	return x
}

func expectSourceCode(t *testing.T, err error, code xsderrors.Code) {
	t.Helper()
	if sourceDiagnostic(t, err, code) == nil {
		t.Fatal("no diagnostic")
	}
}

func TestSchemaSourceLimitCountsLoadedDocuments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a := write("a.xsd", `<xs:schema `+sourceLedgerNS+` targetNamespace="urn:a"><xs:import namespace="urn:b" schemaLocation="b.xsd"/></xs:schema>`)
	b := write("b.xsd", `<xs:schema `+sourceLedgerNS+` targetNamespace="urn:b"/>`)
	missing := write("missing.xsd", `<xs:schema `+sourceLedgerNS+` targetNamespace="urn:m"><xs:import namespace="urn:x" schemaLocation="x1.xsd"/><xs:import namespace="urn:y" schemaLocation="x2.xsd"/><xs:import namespace="urn:z" schemaLocation="x3.xsd"/></xs:schema>`)

	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 2}, xsd.File(a), xsd.File(b)); err != nil {
		t.Fatalf("explicit file also imported, limit 2: %v", err)
	}
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 2}, xsd.File(a)); err != nil {
		t.Fatalf("file and import, limit 2: %v", err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 1}, xsd.File(a))
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 1}, xsd.File(missing)); err != nil {
		t.Fatalf("unavailable targets must not count: %v", err)
	}
}

// commonGraph is a root importing 20 schemas that each import one common
// schema, served from memory or through openers.
type commonGraph struct {
	files              map[string]string
	root               string
	distinct, allReach int64
}

func newCommonGraph() commonGraph {
	g := commonGraph{files: map[string]string{}, root: "root.xsd"}
	var locations []string
	for i := range 20 {
		name := "c" + strconv.Itoa(i)
		locations = append(locations, name)
		g.files[name+".xsd"] = ledgerSchema("urn:"+name, "common")
	}
	g.files["common.xsd"] = ledgerSchema("urn:common")
	g.files["root.xsd"] = ledgerSchema("urn:root", locations...)
	for name, content := range g.files {
		g.distinct += int64(len(content))
		g.allReach += int64(len(content))
		if name == "common.xsd" {
			g.allReach += 19 * int64(len(content))
		}
	}
	return g
}

func (g commonGraph) bytesRoot() xsd.SchemaSource {
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		content, ok := g.files[location]
		if !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.Bytes(location, []byte(content)), nil
	})
	return xsd.Bytes(g.root, []byte(g.files[g.root])).WithResolver(resolver)
}

func (g commonGraph) openerRoot() xsd.SchemaSource {
	opener := func(name string) xsd.SchemaSource {
		return xsd.Open(name, func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(g.files[name])), nil })
	}
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if _, ok := g.files[location]; !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return opener(location), nil
	})
	return opener(g.root).WithResolver(resolver)
}

func TestSchemaSourceBytesLedgerChargesInMemoryRepeatsOnce(t *testing.T) {
	t.Parallel()
	g := newCommonGraph()
	for _, total := range []int64{g.distinct, g.distinct - 1} {
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: total}, g.bytesRoot())
		if total == g.distinct && err != nil {
			t.Fatalf("distinct total %d: %v", total, err)
		}
		if total != g.distinct {
			expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
		}
	}
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 22}, g.bytesRoot()); err != nil {
		t.Fatalf("limit 22: %v", err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 21}, g.bytesRoot())
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}

// fileRoot writes the graph to disk and returns its root as a built-in file
// source.
func (g commonGraph) fileRoot(t *testing.T) xsd.SchemaSource {
	t.Helper()
	return xsd.File(filepath.Join(writeSchemaFiles(t, g.files), g.root))
}

func writeSchemaFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSchemaSourceBytesLedgerChargesFileRepeatsOnce(t *testing.T) {
	t.Parallel()
	g := newCommonGraph()
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: g.distinct}, g.fileRoot(t)); err != nil {
		t.Fatalf("distinct total %d: %v", g.distinct, err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: g.distinct - 1}, g.fileRoot(t))
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSchemaSourceFileExplicitAndReferencedIsChargedOnce(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"a.xsd": ledgerSchema("urn:a", "b"),
		"b.xsd": `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:b"/>`,
	}
	dir := writeSchemaFiles(t, files)
	total := int64(len(files["a.xsd"]) + len(files["b.xsd"]))
	sources := func() []xsd.SchemaSource {
		return []xsd.SchemaSource{xsd.File(filepath.Join(dir, "a.xsd")), xsd.File(filepath.Join(dir, "b.xsd"))}
	}
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: total}, sources()...); err != nil {
		t.Fatalf("budget %d: %v", total, err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: total - 1}, sources()...)
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSchemaSourceFileResolutionContextDoesNotRepeatCharge(t *testing.T) {
	t.Parallel()
	content := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:a"/>`
	path := filepath.Join(writeSchemaFiles(t, map[string]string{"a.xsd": content}), "a.xsd")
	custom := xsd.ResolverFunc(func(_, _ string) (xsd.SchemaSource, error) { return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound })
	sources := func() []xsd.SchemaSource {
		return []xsd.SchemaSource{xsd.File(path), xsd.File(path).WithResolver(custom)}
	}
	size := int64(len(content))
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: size}, sources()...); err != nil {
		t.Fatalf("budget %d: %v", size, err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: size - 1}, sources()...)
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSchemaSourceFileSecondContextStillResolvesDescendants(t *testing.T) {
	t.Parallel()
	const rootText = `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:root"><xs:import namespace="urn:x" schemaLocation="x.xsd"/></xs:schema>`
	dir := writeSchemaFiles(t, map[string]string{"root.xsd": rootText})
	path := filepath.Join(dir, "root.xsd")
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if location != "x.xsd" {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.Bytes(filepath.Join(dir, "x.xsd"), []byte(`<xs:schema `+sourceLedgerNS+` targetNamespace="urn:x"><xs:element name="e" type="xs:string"/></xs:schema>`)), nil
	})
	engine, err := xsd.Compile(xsd.File(path), xsd.File(path).WithResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(strings.NewReader(`<e xmlns="urn:x">v</e>`)); err != nil {
		t.Fatalf("Validate() = %v, want element e resolved through the second context", err)
	}
}

func TestSchemaSourceFileAfterOpenerIsReadOnce(t *testing.T) {
	t.Parallel()
	dContent := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:d"/>`
	rootText := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:root"><xs:import namespace="urn:d" schemaLocation="d.xsd"/><xs:import namespace="urn:d" schemaLocation="d.xsd"/></xs:schema>`
	path := filepath.Join(writeSchemaFiles(t, map[string]string{"d.xsd": dContent}), "d.xsd")
	sources := func() []xsd.SchemaSource {
		resolver := xsd.ResolverFunc(func(_, _ string) (xsd.SchemaSource, error) { return xsd.File(path), nil })
		return []xsd.SchemaSource{
			xsd.Open(path, func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(dContent)), nil }),
			xsd.Bytes("root.xsd", []byte(rootText)).WithResolver(resolver),
		}
	}
	// The explicit opener source is charged once, the first file edge is
	// read and compared once, and the second file edge is not reread.
	budget := 2*int64(len(dContent)) + int64(len(rootText))
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: budget}, sources()...); err != nil {
		t.Fatalf("budget %d: %v", budget, err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: budget - 1}, sources()...)
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSchemaSourceFileContentMismatchAcrossKindsIsReferenceError(t *testing.T) {
	t.Parallel()
	onDisk := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:d"/>`
	other := []byte(`<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:y"/>`)
	path := filepath.Join(writeSchemaFiles(t, map[string]string{"d.xsd": onDisk}), "d.xsd")
	otherOpener := xsd.Open(path, func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(string(other))), nil })
	for name, sources := range map[string][]xsd.SchemaSource{
		"different opener then file": {otherOpener, xsd.File(path)},
		"file then different opener": {xsd.File(path), otherOpener},
		"different bytes then file":  {xsd.Bytes(path, other), xsd.File(path)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.Compile(sources...)
			expectSourceCode(t, err, xsderrors.CodeSchemaReference)
		})
	}
}

// A repeated file edge is not reread, but the reference it carries is still
// checked against the loaded document.
func TestSchemaSourceFileSkippedEdgeStillValidatesReference(t *testing.T) {
	t.Parallel()
	common := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:common"/>`
	good := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:a"><xs:import namespace="urn:common" schemaLocation="common.xsd"/></xs:schema>`
	bad := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:b"><xs:import namespace="urn:wrong" schemaLocation="common.xsd"/></xs:schema>`
	dir := writeSchemaFiles(t, map[string]string{"a.xsd": good, "b.xsd": bad, "common.xsd": common})
	a, b := filepath.Join(dir, "a.xsd"), filepath.Join(dir, "b.xsd")
	_, alone := xsd.Compile(xsd.File(b))
	want := sourceDiagnostic(t, alone, xsderrors.CodeSchemaReference).Code()
	distinct := int64(len(common) + len(good) + len(bad))
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: distinct}, xsd.File(a), xsd.File(b))
	expectSourceCode(t, err, want)
}

func TestSchemaSourceBytesLedgerChargesOpenerRepeatsAgain(t *testing.T) {
	t.Parallel()
	g := newCommonGraph()
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: g.distinct}, g.openerRoot())
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
	if _, compileErr := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: g.allReach}, g.openerRoot()); compileErr != nil {
		t.Fatalf("sum over every reach: %v", compileErr)
	}
	_, err = xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: g.allReach - 1}, g.openerRoot())
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSchemaSourceLimitFailuresAreLocated(t *testing.T) {
	t.Parallel()
	const rootText = "<xs:schema " + sourceLedgerNS + " targetNamespace=\"urn:root\">\n" +
		"  <xs:import namespace=\"urn:a\" schemaLocation=\"a.xsd\"/>\n" +
		"  <xs:import namespace=\"urn:b\" schemaLocation=\"b.xsd\"/>\n" +
		"</xs:schema>"
	files := map[string]string{
		"a.xsd": `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:a"/>`,
		"b.xsd": `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:b"/>`,
	}
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		content, ok := files[location]
		if !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.Bytes(location, []byte(content)), nil
	})
	root := xsd.Bytes("root.xsd", []byte(rootText)).WithResolver(resolver)

	t.Run("total bytes through import", func(t *testing.T) {
		t.Parallel()
		limit := int64(len(rootText) + len(files["a.xsd"]) + len(files["b.xsd"]) - 1)
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: limit}, root)
		x := sourceDiagnostic(t, err, xsderrors.CodeSchemaLimit)
		if !strings.Contains(err.Error(), "MaxSchemaTotalBytes") || x.Path() != "root.xsd" || x.Line() != 3 || x.Column() != 3 {
			t.Fatalf("location = %s:%d:%d (%v), want root.xsd:3:3 naming MaxSchemaTotalBytes", x.Path(), x.Line(), x.Column(), err)
		}
	})
	t.Run("total bytes on explicit source", func(t *testing.T) {
		t.Parallel()
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalBytes: int64(len(rootText) - 1)}, root)
		x := sourceDiagnostic(t, err, xsderrors.CodeSchemaLimit)
		if x.Path() != "root.xsd" {
			t.Fatalf("Path() = %q, want root.xsd (%v)", x.Path(), err)
		}
	})
	t.Run("source count through import of an opener source", func(t *testing.T) {
		t.Parallel()
		openerResolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
			return xsd.Open(location, func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(files[location])), nil
			}), nil
		})
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 2}, xsd.Bytes("root.xsd", []byte(rootText)).WithResolver(openerResolver))
		x := sourceDiagnostic(t, err, xsderrors.CodeSchemaLimit)
		if x.Path() != "root.xsd" || x.Line() != 3 || x.Column() != 3 {
			t.Fatalf("location = %s:%d:%d (%v), want root.xsd:3:3", x.Path(), x.Line(), x.Column(), err)
		}
	})
	t.Run("source count through import", func(t *testing.T) {
		t.Parallel()
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 2}, root)
		x := sourceDiagnostic(t, err, xsderrors.CodeSchemaLimit)
		if !strings.Contains(err.Error(), "MaxSchemaSources") || x.Path() != "root.xsd" || x.Line() != 3 || x.Column() != 3 {
			t.Fatalf("location = %s:%d:%d (%v), want root.xsd:3:3 naming MaxSchemaSources", x.Path(), x.Line(), x.Column(), err)
		}
	})
}

func TestSchemaSourceIdentityContentMismatchIsReferenceError(t *testing.T) {
	t.Parallel()
	const rootText = `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:root"><xs:import namespace="urn:x" schemaLocation="x.xsd"/><xs:import namespace="urn:x" schemaLocation="x.xsd"/></xs:schema>`
	for name, second := range map[string]string{
		"same length":      `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:y"/>`,
		"different length": `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:xx"/>`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			resolver := xsd.ResolverFunc(func(_, _ string) (xsd.SchemaSource, error) {
				calls++
				if calls == 1 {
					return xsd.Bytes("x.xsd", []byte(`<xs:schema `+sourceLedgerNS+` targetNamespace="urn:x"/>`)), nil
				}
				return xsd.Bytes("x.xsd", []byte(second)), nil
			})
			_, err := xsd.CompileWithOptions(xsd.CompileOptions{}, xsd.Bytes("root.xsd", []byte(rootText)).WithResolver(resolver))
			expectSourceCode(t, err, xsderrors.CodeSchemaReference)
		})
	}
}

func TestSchemaSourceIdentityOpenerContentMismatchIsReferenceError(t *testing.T) {
	t.Parallel()
	const rootText = `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:root"><xs:import namespace="urn:x" schemaLocation="x.xsd"/><xs:import namespace="urn:x" schemaLocation="x.xsd"/></xs:schema>`
	calls := 0
	resolver := xsd.ResolverFunc(func(_, _ string) (xsd.SchemaSource, error) {
		calls++
		content := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:x"/>`
		if calls > 1 {
			content = `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:y"/>`
		}
		return xsd.Open("x.xsd", func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(content)), nil }), nil
	})
	_, err := xsd.Compile(xsd.Bytes("root.xsd", []byte(rootText)).WithResolver(resolver))
	expectSourceCode(t, err, xsderrors.CodeSchemaReference)
}

// readCountingCloser counts the bytes read from its body and the closes.
type readCountingCloser struct {
	io.Reader

	read     *int
	closes   *int
	closeErr error
}

func (c readCountingCloser) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	*c.read += n
	return n, err
}

func (c readCountingCloser) Close() error { *c.closes++; return c.closeErr }

func TestOverLimitSchemaSourceIsOpenedOnceAndClosed(t *testing.T) {
	t.Parallel()
	const rootText = `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:root"><xs:import namespace="urn:a" schemaLocation="a.xsd"/><xs:import namespace="urn:b" schemaLocation="b.xsd"/></xs:schema>`
	body := `<xs:schema ` + sourceLedgerNS + ` targetNamespace="urn:a">` + strings.Repeat(" ", 1<<20) + `</xs:schema>`
	run := func(t *testing.T, closeErr error) (opens, closes, read, calls int, err error) {
		t.Helper()
		resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
			calls++
			return xsd.Open(location, func() (io.ReadCloser, error) {
				opens++
				return readCountingCloser{Reader: strings.NewReader(body), read: &read, closes: &closes, closeErr: closeErr}, nil
			}), nil
		})
		_, err = xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 1}, xsd.Bytes("root.xsd", []byte(rootText)).WithResolver(resolver))
		return opens, closes, read, calls, err
	}
	t.Run("closed unread", func(t *testing.T) {
		t.Parallel()
		opens, closes, read, calls, err := run(t, nil)
		expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
		if opens != 1 || closes != 1 || read != 0 || calls != 1 {
			t.Fatalf("opens=%d closes=%d bytes read=%d resolver calls=%d, want 1, 1, 0, 1", opens, closes, read, calls)
		}
	})
	t.Run("close error is joined", func(t *testing.T) {
		t.Parallel()
		closeErr := errors.New("close failed")
		_, _, _, _, err := run(t, closeErr)
		expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
		if !errors.Is(err, closeErr) {
			t.Fatalf("err = %v, want it to wrap the close error", err)
		}
	})
}

// TestSchemaSourceLimitDoesNotRejectReloadedOpenerDocument pins that reaching
// an already loaded opener document at the limit is not a new document.
func TestSchemaSourceLimitDoesNotRejectReloadedOpenerDocument(t *testing.T) {
	t.Parallel()
	g := newCommonGraph()
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 22}, g.openerRoot()); err != nil {
		t.Fatalf("limit 22: %v", err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSources: 21}, g.openerRoot())
	expectSourceCode(t, err, xsderrors.CodeSchemaLimit)
}
