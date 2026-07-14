package compile_test

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/validate"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileInheritedEnumerationRestrictionChain(t *testing.T) {
	t.Parallel()

	const (
		depth            = 100
		enumerationCount = 100
	)
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	schema.WriteString(`<xs:simpleType name="t0"><xs:restriction base="xs:decimal">`)
	for i := range enumerationCount {
		fmt.Fprintf(&schema, `<xs:enumeration value="%d"/>`, i)
	}
	schema.WriteString(`</xs:restriction></xs:simpleType>`)
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&schema, `<xs:simpleType name="t%d"><xs:restriction base="t%d"><xs:minInclusive value="0"/></xs:restriction></xs:simpleType>`, i, i-1)
	}
	fmt.Fprintf(&schema, `<xs:element name="root" type="t%d"/></xs:schema>`, depth)

	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema.String())),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root>0</root>`)
}

func TestSchemaCompileErrorsIncludeLocation(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		needle string
		code   xsderrors.Code
	}{
		{
			name: "pattern",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad">
    <xs:restriction base="xs:string">
      <xs:pattern value="[z-a]"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			needle: `<xs:pattern`,
			code:   xsderrors.CodeSchemaFacet,
		},
		{
			name: "identity",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="child"/></xs:sequence></xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="."/>
      <xs:field xpath="/bad"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			needle: `<xs:field`,
			code:   xsderrors.CodeSchemaIdentity,
		},
		{
			name: "content",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Bad">
    <xs:element name="child"/>
  </xs:complexType>
</xs:schema>`,
			needle: `<xs:element name="child"`,
			code:   xsderrors.CodeSchemaContentModel,
		},
		{
			name: "duplicate schema id",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:simpleContent id="dup">
        <xs:extension id="dup" base="xs:string"/>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			needle: `<xs:extension id="dup"`,
			code:   xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid schema component name",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="0"/>
</xs:schema>`,
			needle: `<xs:attributeGroup name="0"`,
			code:   xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "nested annotation",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation>
    <xs:annotation/>
  </xs:annotation>
</xs:schema>`,
			needle: `<xs:annotation/>`,
			code:   xsderrors.CodeSchemaContentModel,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(test.schema))})
			expectCode(t, err, test.code)
			expectSchemaCompileLine(t, err, lineOf(test.schema, test.needle))
		})
	}
}

func TestMissingIncludedSchemaLocationDoesNotInvalidateSchema(t *testing.T) {
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "missing.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Source{}, xsderrors.ErrSchemaNotFound
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
</xs:schema>`)).WithResolver(resolver)})
	if err != nil {
		t.Fatalf("Compile() error = %v, want nil", err)
	}
}

func TestOpaqueSourceMissingRelativeIncludeDoesNotInvalidateSchema(t *testing.T) {
	t.Parallel()
	root := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/><xs:element name="root"/></xs:schema>`)
	sources := []source.Source{
		source.Bytes("urn:root", root),
		source.Bytes("urn:root", root).WithResolver(func(_, _ string) (source.Source, error) {
			return source.Source{}, xsderrors.ErrSchemaNotFound
		}),
	}
	for _, src := range sources {
		engine, err := compile.Compile(compile.Options{}, []source.Source{src})
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		mustValidateRuntime(t, engine, `<root/>`)
	}
}

func TestLocalAndOpaqueSchemaIdentitiesRemainDistinct(t *testing.T) {
	t.Parallel()
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("./urn:types", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:local"/>`)),
		source.Bytes("urn:types", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:opaque"/>`)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestJoinedMissingResolverErrorIsNotSuppressed(t *testing.T) {
	t.Parallel()
	fatal := errors.New("resolver failed after lookup")
	resolver := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Source{}, errors.Join(xsderrors.ErrSchemaNotFound, fatal)
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
</xs:schema>`)).WithResolver(resolver)})
	if !errors.Is(err, fatal) {
		t.Fatalf("Compile() error = %v, want fatal resolver error", err)
	}
}

func TestMissingResolvedIncludeDoesNotInvalidateSchema(t *testing.T) {
	readErrors := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{name: "not exist", err: os.ErrNotExist},
		{name: "wrapped not exist", err: fmt.Errorf("open: %w", os.ErrNotExist)},
		{name: "schema not found after resolution", err: xsderrors.ErrSchemaNotFound, wantErr: true},
		{name: "wrapped schema not found after resolution", err: fmt.Errorf("open: %w", xsderrors.ErrSchemaNotFound), wantErr: true},
	}
	for _, tt := range readErrors {
		t.Run(tt.name, func(t *testing.T) {
			resolver := source.Resolver(func(_, location string) (source.Source, error) {
				if location != "optional.xsd" {
					return source.Source{}, errors.New("unexpected location " + location)
				}
				return source.Opener("optional.xsd", func() (io.ReadCloser, error) {
					return nil, tt.err
				}), nil
			})
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="optional.xsd"/>
</xs:schema>`)).WithResolver(resolver)})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Compile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMissingResolvedIncludeCleanupErrorIsNotSuppressed(t *testing.T) {
	t.Parallel()
	closeErr := errors.New("close failed after missing open")
	resolver := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Opener("optional.xsd", func() (io.ReadCloser, error) {
			//nolint:nilnil // Exercise the loader's cleanup-error classification boundary.
			return compileCloseErrorReader{Reader: strings.NewReader("schema"), err: closeErr}, os.ErrNotExist
		}), nil
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="optional.xsd"/>
</xs:schema>`)).WithResolver(resolver)})
	if !errors.Is(err, closeErr) {
		t.Fatalf("Compile() error = %v, want cleanup error", err)
	}
}

func TestSchemaSetReferenceRules(t *testing.T) {
	validSchema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	readFailure := errors.New("read failed")
	resolved := func(name, schema string) source.Resolver {
		return func(_, location string) (source.Source, error) {
			if location != name {
				return source.Source{}, errors.New("unexpected location " + location)
			}
			return source.Bytes(name, []byte(schema)), nil
		}
	}
	tests := []struct {
		name     string
		sources  func() []source.Source
		category xsderrors.Category
		code     xsderrors.Code
		message  string
		line     int
	}{
		{
			name:     "source name required",
			sources:  func() []source.Source { return []source.Source{source.Bytes("", validSchema)} },
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaRead,
			message:  "schema source name is required",
		},
		{
			name: "read error",
			sources: func() []source.Source {
				return []source.Source{source.Opener("broken.xsd", func() (io.ReadCloser, error) { return nil, readFailure })}
			},
			category: xsderrors.CategorySchemaParse,
			code:     xsderrors.CodeSchemaRead,
			message:  "read schema broken.xsd",
		},
		{
			name: "include missing location",
			sources: func() []source.Source {
				return []source.Source{source.Bytes("schema.xsd", []byte("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n  <xs:include/>\n</xs:schema>"))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
			message:  "include missing schemaLocation",
			line:     2,
		},
		{
			name: "import without namespace from no-target schema",
			sources: func() []source.Source {
				return []source.Source{source.Bytes("schema.xsd", []byte("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n  <xs:import/>\n</xs:schema>"))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
			message:  "import without namespace requires enclosing schema targetNamespace",
			line:     2,
		},
		{
			name: "empty import namespace",
			sources: func() []source.Source {
				return []source.Source{source.Bytes("schema.xsd", []byte("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" targetNamespace=\"urn:a\">\n  <xs:import namespace=\"\"/>\n</xs:schema>"))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaInvalidAttribute,
			message:  "import namespace cannot be empty",
			line:     2,
		},
		{
			name: "import matches enclosing target",
			sources: func() []source.Source {
				return []source.Source{source.Bytes("schema.xsd", []byte("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" targetNamespace=\"urn:a\">\n  <xs:import namespace=\"urn:a\"/>\n</xs:schema>"))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
			message:  "import namespace cannot match enclosing schema targetNamespace",
			line:     2,
		},
		{
			name: "import target mismatch",
			sources: func() []source.Source {
				schema := "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" targetNamespace=\"urn:a\">\n  <xs:import namespace=\"urn:b\" schemaLocation=\"other.xsd\"/>\n</xs:schema>"
				return []source.Source{source.Bytes("schema.xsd", []byte(schema)).WithResolver(resolved("other.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:c"/>`))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
			message:  "import namespace does not match imported schema targetNamespace",
			line:     2,
		},
		{
			name: "include target mismatch",
			sources: func() []source.Source {
				schema := "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" targetNamespace=\"urn:a\">\n  <xs:include schemaLocation=\"other.xsd\"/>\n</xs:schema>"
				return []source.Source{source.Bytes("schema.xsd", []byte(schema)).WithResolver(resolved("other.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
			message:  "included schema targetNamespace does not match including schema",
			line:     2,
		},
		{
			name: "unimported QName",
			sources: func() []source.Source {
				schema := "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\" xmlns:b=\"urn:b\" targetNamespace=\"urn:a\">\n  <xs:element name=\"root\" type=\"b:Missing\"/>\n</xs:schema>"
				return []source.Source{source.Bytes("schema.xsd", []byte(schema))}
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
			message:  "namespace is not imported: urn:b",
			line:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compile.Compile(compile.Options{}, tt.sources())
			expectCategoryCode(t, err, tt.category, tt.code)
			if !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Compile() error = %v, want message containing %q", err, tt.message)
			}
			if tt.line != 0 {
				expectSchemaCompileLine(t, err, tt.line)
			}
		})
	}
}

func TestSchemaSetValidReferenceRules(t *testing.T) {
	tests := []struct {
		name      string
		root      string
		child     string
		childName string
	}{
		{
			name:      "import no namespace from targeted schema",
			root:      `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:import schemaLocation="child.xsd"/></xs:schema>`,
			child:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
			childName: "child.xsd",
		},
		{
			name:      "foreign import matches imported target",
			root:      `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:import namespace="urn:b" schemaLocation="child.xsd"/></xs:schema>`,
			child:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`,
			childName: "child.xsd",
		},
		{
			name:      "include matching target",
			root:      `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			child:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"/>`,
			childName: "child.xsd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := source.Resolver(func(_, location string) (source.Source, error) {
				if location != tt.childName {
					return source.Source{}, errors.New("unexpected location " + location)
				}
				return source.Bytes(tt.childName, []byte(tt.child)), nil
			})
			if _, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(tt.root)).WithResolver(resolver)}); err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
		})
	}
}

func TestSchemaDirectivesMustPrecedeGlobalDeclarations(t *testing.T) {
	for _, directive := range []string{
		`<xs:include schemaLocation="child.xsd"/>`,
		`<xs:import namespace="urn:child" schemaLocation="child.xsd"/>`,
	} {
		calls := 0
		resolver := source.Resolver(func(_, _ string) (source.Source, error) {
			calls++
			return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
		})
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root">
  <xs:element name="root"/>
  <xs:annotation><xs:documentation>late directive</xs:documentation></xs:annotation>
  ` + directive + `
</xs:schema>`
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema)).WithResolver(resolver)})
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
		expectSchemaCompileLine(t, err, 4)
		if calls != 0 {
			t.Fatalf("resolver calls = %d, want 0", calls)
		}
	}
}

func TestSchemaTopLevelOrderAcceptsLeadingDirectivesAndAnnotations(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root">
  <xs:annotation/>
  <xs:include schemaLocation="child.xsd"/>
  <xs:annotation/>
  <xs:element name="root"/>
  <xs:annotation/>
</xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "child.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
	})
	if _, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root)).WithResolver(resolver)}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestLocalImportRulesPrecedeResolverCalls(t *testing.T) {
	for _, root := range []string{
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root"><xs:import namespace="urn:root" schemaLocation="child.xsd"/></xs:schema>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root"><xs:import namespace="" schemaLocation="child.xsd"/></xs:schema>`,
	} {
		calls := 0
		resolver := source.Resolver(func(_, _ string) (source.Source, error) {
			calls++
			return source.Source{}, errors.New("resolver must not be called")
		})
		if _, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root)).WithResolver(resolver)}); err == nil {
			t.Fatal("Compile() accepted invalid import")
		}
		if calls != 0 {
			t.Fatalf("resolver calls = %d, want 0", calls)
		}
	}
}

func TestInvalidSchemaDefaultsPrecedeResolverCalls(t *testing.T) {
	t.Parallel()
	for _, schema := range []string{
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace=""><xs:include schemaLocation="child.xsd"/></xs:schema>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="invalid"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" blockDefault="invalid"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
	} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()
			calls := 0
			resolver := source.Resolver(func(_, _ string) (source.Source, error) {
				calls++
				return source.Source{}, errors.New("resolver must not be called")
			})
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(schema)).WithResolver(resolver)})
			expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
			if calls != 0 {
				t.Fatalf("resolver calls = %d, want 0", calls)
			}
		})
	}
}

func TestInvalidChildSchemaDefaultsPrecedeDescendantResolverCalls(t *testing.T) {
	t.Parallel()
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`
	const child = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" finalDefault="invalid"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	var calls []string
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		calls = append(calls, location)
		if location == "child.xsd" {
			return source.Bytes("child.xsd", []byte(child)), nil
		}
		return source.Source{}, errors.New("descendant resolver must not be called")
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root)).WithResolver(resolver)})
	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
	if want := []string{"child.xsd"}; !slices.Equal(calls, want) {
		t.Fatalf("resolver calls = %v, want %v", calls, want)
	}
}

func TestReferenceTargetMismatchDoesNotResolveDescendants(t *testing.T) {
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root"><xs:include schemaLocation="child.xsd"/></xs:schema>`
	const child = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	var calls []string
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		calls = append(calls, location)
		switch location {
		case "child.xsd":
			return source.Bytes("child.xsd", []byte(child)), nil
		case "leaf.xsd":
			return source.Source{}, errors.New("unreachable descendant")
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root)).WithResolver(resolver)})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !slices.Equal(calls, []string{"child.xsd"}) {
		t.Fatalf("resolver calls = %v, want direct target only", calls)
	}
}

func TestCachedReferenceTargetMismatchDoesNotActivateResolverContext(t *testing.T) {
	const compatible = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shared"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const incompatible = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const shared = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shared"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	var incompatibleCalls []string
	compatibleResolver := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "shared.xsd":
			return source.Bytes("shared.xsd", []byte(shared)), nil
		case "leaf.xsd":
			return source.Bytes("leaf.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shared"/>`)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	incompatibleResolver := source.Resolver(func(_, location string) (source.Source, error) {
		incompatibleCalls = append(incompatibleCalls, location)
		if location == "shared.xsd" {
			return source.Bytes("shared.xsd", []byte(shared)), nil
		}
		return source.Source{}, errors.New("incompatible resolver reached descendant " + location)
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a-compatible.xsd", []byte(compatible)).WithResolver(compatibleResolver),
		source.Bytes("b-incompatible.xsd", []byte(incompatible)).WithResolver(incompatibleResolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !slices.Equal(incompatibleCalls, []string{"shared.xsd"}) {
		t.Fatalf("incompatible resolver calls = %v, want cached target only", incompatibleCalls)
	}
}

func TestIdentityOnlyReferenceChecksNonExplicitLoadedTarget(t *testing.T) {
	const compatible = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shared"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const incompatible = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const shared = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:shared"/>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "shared.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes("shared.xsd", []byte(shared)), nil
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a-compatible.xsd", []byte(compatible)).WithResolver(resolver),
		source.Bytes("b-incompatible.xsd", []byte(incompatible)),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "included schema targetNamespace does not match including schema") {
		t.Fatalf("Compile() error = %v, want include target mismatch", err)
	}
}

func TestUnresolvedFragmentSchemaLocationIsOptional(t *testing.T) {
	for _, location := range []string{"missing.xsd#fragment", "missing.xsd#"} {
		root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="` + location + `"/><xs:element name="root"/></xs:schema>`
		for _, src := range []source.Source{
			source.Bytes("root.xsd", []byte(root)),
			source.Bytes("root.xsd", []byte(root)).WithResolver(func(_, _ string) (source.Source, error) {
				return source.Source{}, xsderrors.ErrSchemaNotFound
			}),
		} {
			engine, err := compile.Compile(compile.Options{}, []source.Source{src})
			if err != nil {
				t.Fatalf("Compile(%q) error = %v", location, err)
			}
			mustValidateRuntime(t, engine, `<root/>`)
		}
	}
}

func TestUnsupportedLocalSchemaLocationsAreOptional(t *testing.T) {
	for _, location := range []string{"missing.xsd?version=1", "//example.test/missing.xsd", "sub%2Fmissing.xsd"} {
		for _, reference := range []string{
			`<xs:include schemaLocation="` + location + `"/>`,
			`<xs:import namespace="urn:missing" schemaLocation="` + location + `"/>`,
		} {
			root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + reference + `<xs:element name="root"/></xs:schema>`
			sources := []source.Source{source.Bytes("root.xsd", []byte(root))}
			dir := t.TempDir()
			rootPath := filepath.Join(dir, "root.xsd")
			if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
				t.Fatal(err)
			}
			sources = append(sources, source.File(rootPath))
			for _, src := range sources {
				engine, err := compile.Compile(compile.Options{}, []source.Source{src})
				if err != nil {
					t.Fatalf("Compile(%q, %q) error = %v", location, src.Name(), err)
				}
				mustValidateRuntime(t, engine, `<root/>`)
			}
		}
	}
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="sub?version=1"><xs:include schemaLocation="missing.xsd"/><xs:element name="root"/></xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root))})
	if err != nil {
		t.Fatalf("Compile(xml:base) error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root/>`)
}

func TestArbitrarySourceIdentityIsNotParsedAsURIBase(t *testing.T) {
	for _, schema := range []string{
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="sub/"><xs:element name="root"/></xs:schema>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/><xs:element name="root"/></xs:schema>`,
	} {
		engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("x:%zz", []byte(schema))})
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		mustValidateRuntime(t, engine, `<root/>`)
	}
}

func TestUnixFileFallbackPreservesEffectiveBase(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Unix path semantics")
	}

	t.Run("query replaced by relative path", func(t *testing.T) {
		dir := t.TempDir()
		rootPath := filepath.Join(dir, "root.xsd")
		writeCompileTestFile(t, rootPath, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="?version=1"><xs:include schemaLocation="child.xsd"/></xs:schema>`)
		writeCompileTestFile(t, filepath.Join(dir, "child.xsd"), `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="included"/></xs:schema>`)
		engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
		if err != nil {
			t.Fatal(err)
		}
		mustValidateRuntime(t, engine, `<included/>`)
	})

	t.Run("backslash filename", func(t *testing.T) {
		dir := t.TempDir()
		rootPath := filepath.Join(dir, "root.xsd")
		writeCompileTestFile(t, rootPath, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child\name.xsd"/></xs:schema>`)
		writeCompileTestFile(t, filepath.Join(dir, "child\\name.xsd"), `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="included"/></xs:schema>`)
		engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
		if err != nil {
			t.Fatal(err)
		}
		mustValidateRuntime(t, engine, `<included/>`)
	})

	t.Run("network authority does not become local path", func(t *testing.T) {
		dir := t.TempDir()
		rootPath := filepath.Join(dir, "root.xsd")
		childPath := filepath.Join(dir, "child.xsd")
		writeCompileTestFile(t, childPath, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="included"/></xs:schema>`)
		root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="//cdn.example/schemas/"><xs:include schemaLocation="` + childPath + `"/></xs:schema>`
		writeCompileTestFile(t, rootPath, root)
		engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
		if err != nil {
			t.Fatal(err)
		}
		mustNotValidateRuntime(t, engine, `<included/>`, xsderrors.CodeValidationRoot)
	})
}

func TestCustomResolverReceivesNonLocalXMLBase(t *testing.T) {
	child := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`)
	for _, test := range []struct {
		name       string
		sourceName string
		root       string
		wantBase   string
	}{
		{
			name:     "root authority",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="//cdn.example/schemas/"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "//cdn.example/schemas/",
		},
		{
			name:     "root query",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="sub?version=1"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "sub?version=1",
		},
		{
			name:     "directive authority",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include xml:base="//cdn.example/schemas/" schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "//cdn.example/schemas/",
		},
		{
			name:     "directive query",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include xml:base="sub?version=1" schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "sub?version=1",
		},
		{
			name:     "opaque source query",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="?version=1"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "urn:root?version=1",
		},
		{
			name:     "opaque source authority",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="//cdn.example/schemas/"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "urn://cdn.example/schemas/",
		},
		{
			name:     "absolute opaque base",
			root:     `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="urn:catalog:schemas"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase: "urn:catalog:schemas",
		},
		{
			name:       "opaque source empty base preserves query",
			sourceName: "urn:root?version=1",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base=""><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "urn:root?version=1",
		},
		{
			name:       "fragment only is removed",
			sourceName: "https://example.test/schemas/root.xsd",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="#fragment"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "https://example.test/schemas/root.xsd",
		},
		{
			name:       "fragment only preserves parent spelling",
			sourceName: "HTTPS://EXAMPLE.test/%74.xsd#old",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="#new"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "HTTPS://EXAMPLE.test/%74.xsd",
		},
		{
			name:       "empty base preserves parent spelling",
			sourceName: "URN:root%7e?x=%7e#old",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base=""><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "URN:root%7e?x=%7e",
		},
		{
			name:       "opaque query replaces parent fragment",
			sourceName: "URN:root%7e?old=%7e#old",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="?new=%7e#new"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "URN:root%7e?new=%7e",
		},
		{
			name:       "fragment only preserves local hash",
			sourceName: "schemas/root#old.xsd",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="#new"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "schemas/root#old.xsd",
		},
		{
			name:       "directive fragment is removed",
			sourceName: "https://example.test/schemas/root.xsd",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include xml:base="#fragment" schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "https://example.test/schemas/root.xsd",
		},
		{
			name:       "absolute fragment is removed",
			sourceName: "https://example.test/schemas/root.xsd",
			root:       `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="https://cdn.example.test/root.xsd#fragment"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			wantBase:   "https://cdn.example.test/root.xsd",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := source.Resolver(func(base, location string) (source.Source, error) {
				if base != test.wantBase || location != "child.xsd" {
					return source.Source{}, fmt.Errorf("resolver input = %q, %q, want %q, child.xsd", base, location, test.wantBase)
				}
				return source.Bytes("child.xsd", child), nil
			})
			name := test.sourceName
			if name == "" && strings.HasPrefix(test.name, "opaque source") {
				name = "urn:root"
			}
			if name == "" {
				name = "root.xsd"
			}
			engine, err := compile.Compile(compile.Options{}, []source.Source{
				source.Bytes(name, []byte(test.root)).WithResolver(resolver),
			})
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			mustValidateRuntime(t, engine, `<child/>`)
		})
	}
}

func TestFragmentBearingXMLBaseCompiles(t *testing.T) {
	t.Run("without references", func(t *testing.T) {
		root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="#fragment"><xs:element name="root"/></xs:schema>`
		engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root))})
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		mustValidateRuntime(t, engine, `<root/>`)
	})

	t.Run("file include", func(t *testing.T) {
		dir := t.TempDir()
		rootPath := filepath.Join(dir, "root.xsd")
		childPath := filepath.Join(dir, "child.xsd")
		root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="#fragment"><xs:include schemaLocation="child.xsd"/></xs:schema>`
		child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
		if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(childPath, []byte(child), 0o600); err != nil {
			t.Fatal(err)
		}
		engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		mustValidateRuntime(t, engine, `<root/>`)
	})
}

func TestAbsoluteFileReferenceOverridesNonLocalXMLBase(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.xsd")
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`
	if err := os.WriteFile(childPath, []byte(child), 0o600); err != nil {
		t.Fatal(err)
	}
	childURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(childPath)}).String()
	rootPath := filepath.Join(dir, "root.xsd")
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="?version=1"><xs:include schemaLocation="` + childURI + `"/></xs:schema>`
	if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<child/>`)
}

func TestFileFragmentSchemaLocationDoesNotOpenFragmentlessFile(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.xsd")
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd#"/><xs:element name="root"/></xs:schema>`
	if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.xsd"), []byte(`<not-a-schema/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root/>`)
}

func TestMalformedFragmentSchemaLocationRemainsFatal(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="missing.xsd#%zz"/></xs:schema>`
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root))})
	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestXMLNamespaceImportDoesNotCallResolver(t *testing.T) {
	called := false
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		called = true
		return source.Source{}, errors.New("unexpected resolver call for " + location)
	})
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml.xsd"/></xs:schema>`
	if _, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema)).WithResolver(resolver)}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if called {
		t.Fatal("XML namespace import called resolver")
	}
}

func TestResolvedIdentityUsesResolverReturnedSourceName(t *testing.T) {
	main := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:main"><xs:include schemaLocation="common.xsd"/></xs:schema>`
	alias := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="fromAlias"/></xs:schema>`
	competing := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other"><xs:element name="fromCandidate"/></xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "common.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes("alias/common.xsd", []byte(alias)), nil
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("dir/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("dir/common.xsd", []byte(competing)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<m:fromAlias xmlns:m="urn:main"/>`)
}

func TestResolverHandlesSchemaLocationBeforeGenericURIResolution(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:main"><xs:include schemaLocation="relative?query#fragment"/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`
	called := false
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		called = true
		if base != "urn:opaque:root" || location != "relative?query#fragment" {
			return source.Source{}, fmt.Errorf("resolver input = %q, %q", base, location)
		}
		return source.Bytes("urn:cache:child#v1", []byte(child)), nil
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("urn:opaque:root", []byte(root)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !called {
		t.Fatal("Compile() did not call resolver")
	}
	mustValidateRuntime(t, engine, `<m:child xmlns:m="urn:main"/>`)
}

func TestMalformedSchemaLocationPrecedesSuccessfulResolver(t *testing.T) {
	t.Parallel()
	called := false
	resolver := source.Resolver(func(_, _ string) (source.Source, error) {
		called = true
		return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd#%zz"/></xs:schema>`)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if called {
		t.Fatal("Compile() called resolver for malformed schemaLocation")
	}
}

func TestResolverReturnedSourceRequiresIdentity(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`
	resolver := source.Resolver(func(_, _ string) (source.Source, error) { return source.Source{}, nil })
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaRead)
	if !strings.Contains(err.Error(), "without a name") {
		t.Fatalf("Compile() error = %v, want unnamed source diagnostic", err)
	}
}

func TestSameResolvedIdentityRejectsDifferentDocumentContent(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	resolverFor := func(element string) source.Resolver {
		return func(_, _ string) (source.Source, error) {
			doc := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="` + element + `"/></xs:schema>`
			return source.Bytes("shared.xsd", []byte(doc)), nil
		}
	}
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(root)).WithResolver(resolverFor("a")),
		source.Bytes("b.xsd", []byte(root)).WithResolver(resolverFor("b")),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "different document content") {
		t.Fatalf("Compile() error = %v, want identity-content conflict", err)
	}
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || xerr.Path != "b.xsd" || xerr.Column <= 1 {
		t.Fatalf("Compile() location = path %q column %d, want second referring root include", xerr.Path, xerr.Column)
	}
}

func TestSameResolverGraphRejectsDifferentContentForOneIdentity(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="a.xsd"/><xs:include schemaLocation="b.xsd"/></xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		doc := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="` + strings.TrimSuffix(location, ".xsd") + `"/></xs:schema>`
		return source.Bytes("shared.xsd", []byte(doc)), nil
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "different document content") {
		t.Fatalf("Compile() error = %v, want identity-content conflict", err)
	}
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || xerr.Path != "root.xsd" || xerr.Column <= 1 {
		t.Fatalf("Compile() location = path %q column %d, want referring root include", xerr.Path, xerr.Column)
	}
}

func TestCachedSourceIdentityKeepsDistinctFallbackContexts(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.xsd")
	childPath := filepath.Join(dir, "child.xsd")
	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`)
	childData := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="included"/></xs:schema>`
	writeCompileTestFile(t, rootPath, string(rootData))
	writeCompileTestFile(t, childPath, childData)

	for _, sources := range [][]source.Source{
		{source.Bytes(rootPath, rootData), source.File(rootPath).WithResolver(nil)},
		{source.File(rootPath).WithResolver(nil), source.Bytes(rootPath, rootData)},
	} {
		engine, err := compile.Compile(compile.Options{}, sources)
		if err != nil {
			t.Fatal(err)
		}
		mustValidateRuntime(t, engine, `<included/>`)
	}

	sharedPath := filepath.Join(dir, "shared.xsd")
	leafPath := filepath.Join(dir, "leaf.xsd")
	sharedData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`)
	writeCompileTestFile(t, sharedPath, string(sharedData))
	writeCompileTestFile(t, leafPath, childData)
	root := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="memory"/><xs:include schemaLocation="file"/></xs:schema>`)
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "memory":
			return source.Bytes(sharedPath, sharedData), nil
		case "file":
			return source.File(sharedPath), nil
		default:
			return source.Source{}, xsderrors.ErrSchemaNotFound
		}
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", root).WithResolver(resolver)})
	if err != nil {
		t.Fatal(err)
	}
	mustValidateRuntime(t, engine, `<included/>`)
}

func TestCanonicalAliasesPreserveReturnedNamesAsDescendantBases(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="one"/><xs:include schemaLocation="two"/></xs:schema>`
	shared := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	leaf := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	var leafBases []string
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		switch location {
		case "one":
			return source.Bytes("dir/../shared.xsd", []byte(shared)), nil
		case "two":
			return source.Bytes("shared.xsd", []byte(shared)), nil
		case "leaf.xsd":
			leafBases = append(leafBases, base)
			return source.Bytes("leaf.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, xsderrors.ErrSchemaNotFound
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	slices.Sort(leafBases)
	if want := []string{"dir/../shared.xsd", "shared.xsd"}; !slices.Equal(leafBases, want) {
		t.Fatalf("resolver descendant bases = %v, want %v", leafBases, want)
	}
}

func TestSameResolvedIdentityRejectsDifferentDescendantIdentities(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	shared := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	leaf := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	resolverFor := func(suffix string) source.Resolver {
		return func(_, location string) (source.Source, error) {
			switch location {
			case "shared.xsd":
				return source.Bytes("shared.xsd", []byte(shared)), nil
			case "leaf.xsd":
				return source.Bytes("leaf-"+suffix+".xsd", []byte(leaf)), nil
			default:
				return source.Source{}, xsderrors.ErrSchemaNotFound
			}
		}
	}
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(root)).WithResolver(resolverFor("a")),
		source.Bytes("b.xsd", []byte(root)).WithResolver(resolverFor("b")),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "different document identities across resolver contexts") {
		t.Fatalf("Compile() error = %v, want descendant identity conflict", err)
	}
}

func TestResolverMissDoesNotBindTentativeDescendantIdentity(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	shared := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	leaf := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="leaf"/></xs:schema>`
	resolverFor := func(missingLeaf bool) source.Resolver {
		return func(_, location string) (source.Source, error) {
			switch location {
			case "shared.xsd":
				return source.Bytes("shared.xsd", []byte(shared)), nil
			case "leaf.xsd":
				if missingLeaf {
					return source.Source{}, xsderrors.ErrSchemaNotFound
				}
				return source.Bytes("catalog/leaf.xsd", []byte(leaf)), nil
			default:
				return source.Source{}, errors.New("unexpected location " + location)
			}
		}
	}
	for _, test := range []struct {
		name         string
		firstMissing bool
	}{
		{name: "miss then success", firstMissing: true},
		{name: "success then miss", firstMissing: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := compile.Compile(compile.Options{}, []source.Source{
				source.Bytes("a.xsd", []byte(root)).WithResolver(resolverFor(test.firstMissing)),
				source.Bytes("b.xsd", []byte(root)).WithResolver(resolverFor(!test.firstMissing)),
			})
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			mustValidateRuntime(t, engine, `<leaf/>`)
		})
	}
}

func TestLateLoadedGenericReferenceContributesTargetContext(t *testing.T) {
	rootA := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	rootB := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"><xs:include schemaLocation="alias.xsd"/></xs:schema>`
	shared := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="item"/></xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "alias.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes("shared.xsd", []byte(shared)), nil
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(rootA)),
		source.Bytes("b.xsd", []byte(rootB)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<item xmlns="urn:a"/>`)
	mustValidateRuntime(t, engine, `<item xmlns="urn:b"/>`)
}

func TestLateLoadedGenericReferenceValidatesTargetNamespace(t *testing.T) {
	rootA := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	rootB := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"><xs:include schemaLocation="alias.xsd"/></xs:schema>`
	shared := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"/>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "alias.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes("shared.xsd", []byte(shared)), nil
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(rootA)),
		source.Bytes("b.xsd", []byte(rootB)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "included schema targetNamespace does not match including schema") {
		t.Fatalf("Compile() error = %v, want late include target mismatch", err)
	}
}

func TestLateLoadedGenericCandidateConflictsWithSuccessfulBinding(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	shared := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	leaf := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	resolverA := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "shared.xsd":
			return source.Bytes("shared.xsd", []byte(shared)), nil
		case "leaf.xsd":
			return source.Bytes("catalog/leaf.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	resolverB := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "shared.xsd":
			return source.Bytes("shared.xsd", []byte(shared)), nil
		case "leaf.xsd":
			return source.Source{}, xsderrors.ErrSchemaNotFound
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	triggerRoot := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="trigger.xsd"/></xs:schema>`
	trigger := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="alias.xsd"/></xs:schema>`
	resolverC := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "trigger.xsd":
			return source.Bytes("trigger.xsd", []byte(trigger)), nil
		case "alias.xsd":
			return source.Bytes("leaf.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(root)).WithResolver(resolverA),
		source.Bytes("b.xsd", []byte(root)).WithResolver(resolverB),
		source.Bytes("c.xsd", []byte(triggerRoot)).WithResolver(resolverC),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "different document identities across resolver contexts") {
		t.Fatalf("Compile() error = %v, want late identity conflict", err)
	}
}

func TestSchemaLoaderCanonicalizesURIIdentityComponents(t *testing.T) {
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	for _, names := range [][2]string{
		{"https://example.test/schema.xsd?q=~", "https://example.test/schema.xsd?q=%7e"},
		{"urn:item~", "urn:item%7E"},
		{"https://example.test/schema.xsd#~", "https://example.test/schema.xsd#%7e"},
	} {
		if _, err := compile.Compile(compile.Options{}, []source.Source{
			source.Bytes(names[0], schema),
			source.Bytes(names[1], schema),
		}); err != nil {
			t.Errorf("Compile(%q, %q) error = %v", names[0], names[1], err)
		}
	}
}

func TestSchemaLoaderRejectsConflictingURIIdentityAliases(t *testing.T) {
	first := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="first"/></xs:schema>`)
	second := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="second"/></xs:schema>`)
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("https://example.test/schema.xsd?q=~", first),
		source.Bytes("https://example.test/schema.xsd?q=%7e", second),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "schema source identity resolves to different document content") {
		t.Fatalf("Compile() error = %v, want identity-content conflict", err)
	}
}

func TestSchemaLoaderKeepsEmptyAuthorityIdentityDistinct(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("foo:", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:no-authority"/>`)),
		source.Bytes("foo://", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:empty-authority"/>`)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSchemaLoaderRejectsConflictingCanonicalSource(t *testing.T) {
	first := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="first"/></xs:schema>`
	second := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="second"/></xs:schema>`
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("dir/../schema.xsd", []byte(first)),
		source.Bytes("schema.xsd", []byte(second)),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "schema source identity resolves to different document content") {
		t.Fatalf("Compile() error = %v, want identity content conflict", err)
	}
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || xerr.Path != "schema.xsd" {
		t.Fatalf("Compile() error path = %#v, want second source schema.xsd", err)
	}
}

func TestCachedSchemaIdentityRetainsResolverContext(t *testing.T) {
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const shared = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	const leaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="leaf"/></xs:schema>`

	resolverA := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "shared.xsd":
			return source.Bytes("shared.xsd", []byte(shared)), nil
		case "leaf.xsd":
			return source.Bytes("leaf-a.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	var resolverBCalls []string
	resolverB := source.Resolver(func(_, location string) (source.Source, error) {
		resolverBCalls = append(resolverBCalls, location)
		switch location {
		case "shared.xsd":
			return source.Opener("shared.xsd", func() (io.ReadCloser, error) { return nil, os.ErrNotExist }), nil
		case "leaf.xsd":
			return source.Bytes("leaf-b.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(root)).WithResolver(resolverA),
		source.Bytes("b.xsd", []byte(root)).WithResolver(resolverB),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !slices.Contains(resolverBCalls, "leaf.xsd") {
		t.Fatalf("resolver B calls = %v, want cached document descendant", resolverBCalls)
	}
	if !strings.Contains(err.Error(), "different document identities across resolver contexts") {
		t.Fatalf("Compile() error = %v, want descendant identity conflict", err)
	}
}

func TestCachedSchemaIdentityCleanupErrorIsNotSuppressed(t *testing.T) {
	t.Parallel()
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const shared = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	resolverA := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Bytes("shared.xsd", []byte(shared)), nil
	})
	closeErr := errors.New("close failed while verifying cached source")
	resolverB := source.Resolver(func(_, _ string) (source.Source, error) {
		return source.Opener("shared.xsd", func() (io.ReadCloser, error) {
			//nolint:nilnil // Exercise cached-source cleanup after a missing open.
			return compileCloseErrorReader{Reader: strings.NewReader(shared), err: closeErr}, os.ErrNotExist
		}), nil
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(root)).WithResolver(resolverA),
		source.Bytes("b.xsd", []byte(root)).WithResolver(resolverB),
	})
	if !errors.Is(err, closeErr) {
		t.Fatalf("Compile() error = %v, want cached-source cleanup error", err)
	}
}

func TestCachedSchemaIdentityResolverContextCountsReferences(t *testing.T) {
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="shared.xsd"/></xs:schema>`
	const shared = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/></xs:schema>`
	const leaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	resolver := func(missingShared bool) source.Resolver {
		return func(_, location string) (source.Source, error) {
			switch location {
			case "shared.xsd":
				if missingShared {
					return source.Opener("shared.xsd", func() (io.ReadCloser, error) { return nil, os.ErrNotExist }), nil
				}
				return source.Bytes("shared.xsd", []byte(shared)), nil
			case "leaf.xsd":
				return source.Bytes("leaf.xsd", []byte(leaf)), nil
			default:
				return source.Source{}, errors.New("unexpected location " + location)
			}
		}
	}
	sources := func() []source.Source {
		return []source.Source{
			source.Bytes("a.xsd", []byte(root)).WithResolver(resolver(false)),
			source.Bytes("b.xsd", []byte(root)).WithResolver(resolver(true)),
		}
	}
	if _, err := compile.Compile(compile.Options{MaxSchemaReferences: 4}, sources()); err != nil {
		t.Fatalf("Compile(exact references) error = %v", err)
	}
	_, err := compile.Compile(compile.Options{MaxSchemaReferences: 3}, sources())
	expectCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSchemaLoaderMatchesCanonicalURLIdentity(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="%74ypes.xsd"/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="canonical"/></xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("https://EXAMPLE.test/main.xsd", []byte(root)),
		source.Bytes("https://example.test/types.xsd", []byte(child)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<canonical/>`)
}

func TestFileSourceResolvesFromPathContainingURIDelimiters(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Windows file names cannot contain these URI delimiters")
	}
	dir := filepath.Join(t.TempDir(), "schemas#revision?one")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(dir, "root.xsd")
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="local"/></xs:schema>`
	if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.xsd"), []byte(child), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<local/>`)
}

func TestFileSourceResolvesFromRelativePathContainingColon(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Windows file names cannot contain colons")
	}
	//nolint:usetesting // The path must be relative to the package working directory.
	dir, err := os.MkdirTemp(".", "schema:relative-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			t.Errorf("RemoveAll(%q) error = %v", dir, removeErr)
		}
	})
	rootPath := filepath.Join(dir, "root.xsd")
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="relative"/></xs:schema>`
	if writeErr := os.WriteFile(rootPath, []byte(root), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile(filepath.Join(dir, "child.xsd"), []byte(child), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<relative/>`)
}

func TestEmptyIncludeLocationResolvesCurrentDocument(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="   "/><xs:element name="root"/></xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root/>`)
}

func TestEmptyIncludeLocationResolvesXMLBaseWithFileSource(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.xsd")
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include xml:base="child.xsd" schemaLocation=""/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="from-child"/></xs:schema>`
	if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.xsd"), []byte(child), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.File(rootPath)})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<from-child/>`)
}

func TestQueryOnlyXMLBaseUsesRFC2396ForGenericIdentity(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xml:base="?y"><xs:include schemaLocation=""/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="from-child"/></xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("http://a/b/c/d;p?q", []byte(root)),
		source.Bytes("http://a/b/c/?y", []byte(child)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<t:from-child xmlns:t="urn:test"/>`)
}

func TestFileSourceResolverMissFallsBackToLocalInclude(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.xsd")
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`
	child := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="from-child"/></xs:schema>`
	if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.xsd"), []byte(child), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.File(rootPath).WithResolver(func(_, _ string) (source.Source, error) {
			return source.Source{}, xsderrors.ErrSchemaNotFound
		}),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<from-child/>`)
}

func TestSchemaLoaderResolvesBreadthFirst(t *testing.T) {
	var calls []string
	documents := map[string]string{
		"a.xsd":      `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf-a.xsd"/></xs:schema>`,
		"b.xsd":      `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf-b.xsd"/></xs:schema>`,
		"leaf-a.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
		"leaf-b.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
	}
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		calls = append(calls, location)
		schema, ok := documents[location]
		if !ok {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes(location, []byte(schema)), nil
	})
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="a.xsd"/><xs:include schemaLocation="b.xsd"/></xs:schema>`
	if _, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root)).WithResolver(resolver)}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	want := []string{"a.xsd", "b.xsd", "leaf-a.xsd", "leaf-b.xsd"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("resolver calls = %v, want %v", calls, want)
	}
}

func TestResolvedSourceNameAndResolverErrorsRemainStructured(t *testing.T) {
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="child.xsd"/>
</xs:schema>`
	tests := []struct {
		name     string
		resolver source.Resolver
		category xsderrors.Category
		message  string
	}{
		{
			name: "resolver error",
			resolver: func(_, _ string) (source.Source, error) {
				return source.Source{}, errors.New("resolver failed")
			},
			category: xsderrors.CategorySchemaParse,
			message:  "resolve schema child.xsd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("root.xsd", []byte(root)).WithResolver(tt.resolver)})
			expectCategoryCode(t, err, tt.category, xsderrors.CodeSchemaRead)
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || xerr.Path != "root.xsd" || xerr.Line != 2 || xerr.Column == 0 {
				t.Fatalf("Compile() error = %#v, want root.xsd:2 with a column", err)
			}
			if !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Compile() error = %v, want message containing %q", err, tt.message)
			}
		})
	}
}

func TestSchemaReferenceErrorsFollowSourceNameOrder(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/../a.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include/></xs:schema>`)),
		source.Bytes("a/../b.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace=""/></xs:schema>`)),
	})
	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
}

func TestLocalReferenceErrorsPrecedeResolvedEdgeErrors(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:import namespace="urn:a"/></xs:schema>`)),
		source.Bytes("z.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:z"><xs:include schemaLocation="child.xsd"/></xs:schema>`)),
		source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:child"/>`)),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "import namespace cannot match enclosing schema targetNamespace") {
		t.Fatalf("Compile() error = %v, want local import error", err)
	}
}

func TestChameleonIncludesCloneTransitivelyForMultipleTargets(t *testing.T) {
	const common = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="leaf.xsd"/>
  <xs:element name="common" type="xs:string"/>
</xs:schema>`
	const leaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="leaf" type="xs:string"/>
</xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "common.xsd":
			return source.Bytes("common.xsd", []byte(common)), nil
		case "leaf.xsd":
			return source.Bytes("leaf.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	root := func(target string) source.Source {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`
		return source.Bytes(strings.TrimPrefix(target, "urn:")+".xsd", []byte(schema)).WithResolver(resolver)
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{root("urn:b"), root("urn:a")})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, doc := range []string{
		`<common xmlns="urn:a">a</common>`,
		`<leaf xmlns="urn:a">a</leaf>`,
		`<common xmlns="urn:b">b</common>`,
		`<leaf xmlns="urn:b">b</leaf>`,
	} {
		mustValidateRuntime(t, engine, doc)
	}
	mustNotValidateRuntime(t, engine, `<common/>`, xsderrors.CodeValidationRoot)
	mustNotValidateRuntime(t, engine, `<leaf/>`, xsderrors.CodeValidationRoot)
}

func TestChameleonIncludesInstantiateAbsentAndNonEmptyTargetsTransitively(t *testing.T) {
	const common = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="leaf.xsd"/>
  <xs:element name="common" type="xs:string"/>
</xs:schema>`
	const leaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="leaf" type="xs:string"/>
</xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "common.xsd":
			return source.Bytes("common.xsd", []byte(common)), nil
		case "leaf.xsd":
			return source.Bytes("leaf.xsd", []byte(leaf)), nil
		default:
			return source.Source{}, errors.New("unexpected location " + location)
		}
	})
	root := func(name, target string) source.Source {
		targetAttribute := ""
		if target != "" {
			targetAttribute = ` targetNamespace="` + target + `"`
		}
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"` + targetAttribute + `>
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`
		return source.Bytes(name, []byte(schema)).WithResolver(resolver)
	}
	for _, tt := range []struct {
		name       string
		absentName string
		namedName  string
	}{
		{name: "absent target assigned first", absentName: "a-absent.xsd", namedName: "z-named.xsd"},
		{name: "non-empty target assigned first", absentName: "z-absent.xsd", namedName: "a-named.xsd"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := compile.Compile(compile.Options{}, []source.Source{
				root(tt.absentName, ""),
				root(tt.namedName, "urn:test"),
			})
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			for _, doc := range []string{
				`<common>absent</common>`,
				`<leaf>absent</leaf>`,
				`<common xmlns="urn:test">named</common>`,
				`<leaf xmlns="urn:test">named</leaf>`,
			} {
				mustValidateRuntime(t, engine, doc)
			}
		})
	}
}

func TestExplicitChameleonSourceRetainsAbsentTargetContext(t *testing.T) {
	const common = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="shared" type="xs:string"/>
</xs:schema>`
	const named = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:named">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`
	sources := func() []source.Source {
		return []source.Source{
			source.Bytes("common.xsd", []byte(common)),
			source.Bytes("named.xsd", []byte(named)),
		}
	}
	engine, err := compile.Compile(compile.Options{MaxSchemaTargetContexts: 3}, sources())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<shared>absent</shared>`)
	mustValidateRuntime(t, engine, `<shared xmlns="urn:named">named</shared>`)

	_, err = compile.Compile(compile.Options{MaxSchemaTargetContexts: 2}, sources())
	expectCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestImportedAbsentTargetChecksTransitiveIncludeTarget(t *testing.T) {
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root">
  <xs:import schemaLocation="child.xsd"/>
</xs:schema>`
	const child = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="foreign.xsd"/>
</xs:schema>`
	const foreign = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:foreign"/>`
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)),
		source.Bytes("child.xsd", []byte(child)),
		source.Bytes("foreign.xsd", []byte(foreign)),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	if !strings.Contains(err.Error(), "included schema targetNamespace does not match including schema") {
		t.Fatalf("Compile() error = %v, want include target mismatch", err)
	}
}

func TestResolverImportAndNamedIncludeInstantiateSharedChameleonTransitively(t *testing.T) {
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:named">
  <xs:import schemaLocation="imported.xsd"/>
  <xs:include schemaLocation="shared.xsd"/>
</xs:schema>`
	documents := map[string]string{
		"imported.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="middle.xsd"/>
</xs:schema>`,
		"middle.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="shared.xsd"/>
</xs:schema>`,
		"shared.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="shared" type="xs:string"/>
</xs:schema>`,
	}
	var resolver source.Resolver
	resolver = func(_, location string) (source.Source, error) {
		schema, ok := documents[location]
		if !ok {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Bytes(location, []byte(schema)).WithResolver(resolver), nil
	}
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<shared>absent</shared>`)
	mustValidateRuntime(t, engine, `<shared xmlns="urn:named">named</shared>`)
}

func TestAbsentTargetIncludeCycleEstablishesRootContext(t *testing.T) {
	const a = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`
	const b = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="a.xsd"/>
  <xs:element name="b" type="xs:string"/>
</xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("a.xsd", []byte(a)),
		source.Bytes("b.xsd", []byte(b)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<a>one</a>`)
	mustValidateRuntime(t, engine, `<b>two</b>`)
}

func TestByteIdenticalSchemasResolveSourceRelativeIncludesIndependently(t *testing.T) {
	const main = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if location != "common.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		switch base {
		case "a/main.xsd":
			return source.Bytes("a/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a"/>
</xs:schema>`)), nil
		case "b/main.xsd":
			return source.Bytes("b/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="b"/>
</xs:schema>`)), nil
		default:
			return source.Source{}, errors.New("unexpected base " + base)
		}
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("a/main.xsd", []byte(main)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	mustValidateRuntime(t, engine, `<t:a xmlns:t="urn:test"/>`)
	mustValidateRuntime(t, engine, `<t:b xmlns:t="urn:test"/>`)
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:test">ok</t:root>`)
}

func TestByteIdenticalSourceRelativeIncludesCompileDeclarationsOnce(t *testing.T) {
	const main = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`
	const common = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="dup"/>
</xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if location != "common.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		switch base {
		case "a/main.xsd":
			return source.Bytes("a/common.xsd", []byte(common)), nil
		case "b/main.xsd":
			return source.Bytes("b/common.xsd", []byte(common)), nil
		default:
			return source.Source{}, errors.New("unexpected base " + base)
		}
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("a/main.xsd", []byte(main)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<t:dup xmlns:t="urn:test"/>`)
}

func TestByteIdenticalNoTargetSourcesCompileDeclarationsOnce(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/schema.xsd", []byte(schema)),
		source.Bytes("a/schema.xsd", []byte(schema)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root/>`)
}

func TestIPv6ZoneCaseKeepsSourceIdentitiesDistinct(t *testing.T) {
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("http://[fe80::1%25ZoneA]/schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:element name="a"/></xs:schema>`)),
		source.Bytes("http://[fe80::1%25zonea]/schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"><xs:element name="b"/></xs:schema>`)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<a:a xmlns:a="urn:a"/>`)
	mustValidateRuntime(t, engine, `<b:b xmlns:b="urn:b"/>`)
}

func TestByteIdenticalSameTargetDuplicateKeepsImportGraphValidation(t *testing.T) {
	const main = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:main">
  <xs:import namespace="urn:dep" schemaLocation="dep.xsd"/>
</xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if location != "dep.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		switch base {
		case "a/main.xsd":
			return source.Bytes("a/dep.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:dep"/>`)), nil
		case "b/main.xsd":
			return source.Bytes("b/dep.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:wrong"/>`)), nil
		default:
			return source.Source{}, errors.New("unexpected base " + base)
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("a/main.xsd", []byte(main)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestByteIdenticalSameTargetSourcesCompileIdentityOnce(t *testing.T) {
	const schema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="code" type="xs:string" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemCode">
      <xs:selector xpath="item"/>
      <xs:field xpath="@code"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/schema.xsd", []byte(schema)),
		source.Bytes("a/schema.xsd", []byte(schema)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:test"><item code="a"/><item code="b"/></t:root>`)
	mustNotValidateRuntime(t, engine, `<t:root xmlns:t="urn:test"><item code="a"/><item code="a"/></t:root>`, xsderrors.CodeValidationIdentity)
}

func expectSchemaCompileLine(t *testing.T, err error, line int) {
	t.Helper()
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error type = %T, want *xsderrors.Error", err)
	}
	if x.Category != xsderrors.CategorySchemaCompile {
		t.Fatalf("error category = %s, want %s", x.Category, xsderrors.CategorySchemaCompile)
	}
	if x.Line != line || x.Column == 0 {
		t.Fatalf("error location = %d:%d, want line %d and non-zero column", x.Line, x.Column, line)
	}
}

func lineOf(s, needle string) int {
	before, _, ok := strings.Cut(s, needle)
	if !ok {
		return 0
	}
	return strings.Count(before, "\n") + 1
}

func TestInvalidSchemaContentOrdering(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a"/>
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad">
    <xs:attribute name="a"/>
    <xs:complexType/>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a"/>
    <xs:annotation/>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad"><xs:complexType name="localName"/></xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad" block="substitution"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent mixed="true"><xs:extension base="base"/></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"/><xs:annotation/></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:sequence/><xs:choice/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"><xs:all><xs:element name="b"/></xs:all></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestInvalidAnnotationStructureIsSchemaError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:annotation/></xs:annotation>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:annotation/>
    <xs:annotation/>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation foo="bar"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:documentation xml:lang=" "/></xs:annotation>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g">
    <xs:attribute name="a"/>
    <xs:annotation/>
  </xs:attributeGroup>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestComplexContentCannotDeriveFromItself(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="bad"><xs:sequence><xs:element name="child"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestSimpleTypeCannotRestrictAnySimpleType(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad"><xs:restriction base="xs:anySimpleType"/></xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestSimpleAndComplexTypesShareNames(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="dup"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="dup"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaDuplicate)
}

func TestAnonymousComplexDerivationWaitsForCompilingBase(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="Base"/>
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="child" minOccurs="0">
        <xs:complexType>
          <xs:complexContent>
            <xs:extension base="Base">
              <xs:sequence>
                <xs:element name="leaf" type="xs:string"/>
              </xs:sequence>
            </xs:extension>
          </xs:complexContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	mustValidateRuntime(t, engine, `<root><child><child><leaf>x</leaf></child><leaf>x</leaf></child></root>`)
}

func TestSubstitutionImplicitTypeInheritanceWaitsForCompleteHead(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="member" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="member" substitutionGroup="head"/>
</xs:schema>`)

	mustValidateRuntime(t, engine, `<head><member/></head>`)
}

func TestSubstitutionInheritedTypeReplaysValueConstraint(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head" default="not-int"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaFacet)
}

func TestDuplicateSingleValueFacetRejectedPerRestrictionStep(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad">
    <xs:restriction base="xs:string">
      <xs:minLength value="1"/>
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaFacet)
}

func TestRepeatedPatternAndEnumerationFacetsRemainLegal(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="code">
    <xs:restriction base="xs:string">
      <xs:pattern value="A"/>
      <xs:pattern value="B"/>
      <xs:enumeration value="A"/>
      <xs:enumeration value="B"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="code"/>
</xs:schema>`)
}

func TestValidationComparesRawLexicalElementNames(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:element name="root">
    <xs:complexType/>
  </xs:element>
</xs:schema>`)

	err := validateWithRuntime(engine, `<p:root xmlns:p="urn:test" xmlns:q="urn:test"></q:root>`)
	expectCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "end element </q:root> does not match start element <p:root>") {
		t.Fatalf("Validate() error = %v, want raw lexical mismatch", err)
	}

	err = validateWithRuntime(engine, `<p:root xmlns:p="urn:test"></q:root>`)
	expectCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "unbound namespace prefix q") {
		t.Fatalf("Validate() error = %v, want namespace resolution error", err)
	}
}

func TestImportedXMLNamespaceSchemaDefersToBuiltinAttributes(t *testing.T) {
	xmlSchema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://www.w3.org/XML/1998/namespace">
  <xs:attribute name="lang" type="xs:string"/>
</xs:schema>`
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:xml="http://www.w3.org/XML/1998/namespace">
  <xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml.xsd"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="xml:lang"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema)),
		source.Bytes("xml.xsd", []byte(xmlSchema))})

	if err != nil {
		t.Fatal(err)
	}
	mustValidateRuntime(t, engine, `<root xml:lang="en"/>`)
	mustNotValidateRuntime(t, engine, `<root xml:lang="@@"/>`, xsderrors.CodeValidationFacet)
}

func TestMissingElementTypeInvalidatesOnlyThatElement(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="good" type="xs:int"/>
  <xs:element name="bad" type="absent"/>
	</xs:schema>`)
	mustValidateRuntime(t, engine, `<good>1</good>`)
	mustNotValidateRuntime(t, engine, `<bad>1</bad>`, xsderrors.CodeValidationElement)
}

func TestUnionMissingMemberInvalidatesOnlyAffectedType(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badUnion"><xs:union memberTypes="absent xs:int"/></xs:simpleType>
  <xs:element name="bad" type="badUnion"/>
  <xs:element name="good" type="xs:int"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<good>1</good>`)
	mustNotValidateRuntime(t, engine, `<bad>1</bad>`, xsderrors.CodeValidationElement)
}

func TestUnionRejectsMissingSchemaNamespaceMember(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad"><xs:union memberTypes="xs:absent"/></xs:simpleType>
</xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaReference)
	expectSchemaCompileLine(t, err, 3)
}

func TestUnionFlattensUnionValuedMembers(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="baseUnion"><xs:union memberTypes="xs:int"/></xs:simpleType>
  <xs:simpleType name="constrainedUnion">
    <xs:restriction base="baseUnion"><xs:enumeration value="1"/></xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="namedOuter"><xs:union memberTypes="constrainedUnion"/></xs:simpleType>
  <xs:simpleType name="anonymousOuter"><xs:union>
    <xs:simpleType><xs:restriction base="baseUnion"><xs:enumeration value="1"/></xs:restriction></xs:simpleType>
  </xs:union></xs:simpleType>
  <xs:element name="named" type="namedOuter"/>
  <xs:element name="anonymous" type="anonymousOuter"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<named>2</named>`)
	mustValidateRuntime(t, engine, `<anonymous>2</anonymous>`)
}

func TestUnionCanonicalizesRepeatedFlattenedMembers(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="inner"><xs:union memberTypes="xs:int xs:string xs:int"/></xs:simpleType>
  <xs:simpleType name="outer"><xs:union memberTypes="inner inner xs:boolean inner"/></xs:simpleType>
  <xs:element name="value" type="outer"/>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	outer := simpleBuildTypeIDByName(t, build, "outer")
	if got := len(build.SimpleTypes[outer].Union); got != 3 {
		t.Fatalf("outer flattened members = %d, want 3 unique members", got)
	}
	engine := mustCompileRuntime(t, schema)
	mustValidateRuntime(t, engine, `<value>1</value>`)
	mustValidateRuntime(t, engine, `<value>true</value>`)
}

func TestSimpleUnionMemberEntryLimit(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="u0"><xs:union memberTypes="xs:int"/></xs:simpleType>
  <xs:simpleType name="u1"><xs:union memberTypes="u0 xs:string"/></xs:simpleType>
  <xs:simpleType name="u2"><xs:union memberTypes="u1 xs:boolean"/></xs:simpleType>
</xs:schema>`
	sourceFor := func() []source.Source { return []source.Source{source.Bytes("schema.xsd", []byte(schema))} }
	if _, err := compile.Compile(compile.Options{MaxSimpleUnionMemberEntries: 6}, sourceFor()); err != nil {
		t.Fatalf("Compile(exact union member entries) error = %v", err)
	}
	_, err := compile.Compile(compile.Options{MaxSimpleUnionMemberEntries: 5}, sourceFor())
	expectCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestSimpleUnionMemberEntryLimitIncludesRestrictions(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="u"><xs:union memberTypes="xs:int xs:string"/></xs:simpleType>
  <xs:simpleType name="r1"><xs:restriction base="u"/></xs:simpleType>
  <xs:simpleType name="r2"><xs:restriction base="r1"/></xs:simpleType>
</xs:schema>`
	sourceFor := func() []source.Source { return []source.Source{source.Bytes("schema.xsd", []byte(schema))} }
	if _, err := compile.Compile(compile.Options{MaxSimpleUnionMemberEntries: 6}, sourceFor()); err != nil {
		t.Fatalf("Compile(exact restricted-union member entries) error = %v", err)
	}
	_, err := compile.Compile(compile.Options{MaxSimpleUnionMemberEntries: 5}, sourceFor())
	expectCode(t, err, xsderrors.CodeSchemaLimit)
}

func TestRestrictionsOfUnavailableTypesDeferFacetValues(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badUnion"><xs:union memberTypes="absent xs:int"/></xs:simpleType>
  <xs:simpleType name="restrictedUnion"><xs:restriction base="badUnion"><xs:enumeration value="x"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:simpleType name="restrictedList"><xs:restriction base="badList"><xs:enumeration value="x"/></xs:restriction></xs:simpleType>
  <xs:complexType name="badContent"><xs:simpleContent><xs:extension base="badUnion"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="restrictedContent"><xs:simpleContent><xs:restriction base="badContent"><xs:enumeration value="x"/></xs:restriction></xs:simpleContent></xs:complexType>
  <xs:element name="union" type="restrictedUnion"/>
  <xs:element name="list" type="restrictedList"/>
  <xs:element name="content" type="restrictedContent"/>
  <xs:element name="good" type="xs:int"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<good>1</good>`)
	for _, doc := range []string{`<union>x</union>`, `<list>x</list>`, `<content>x</content>`} {
		mustNotValidateRuntime(t, engine, doc, xsderrors.CodeValidationElement)
	}
}

func TestUnavailableRestrictionsRetainDecidableFacetState(t *testing.T) {
	for _, restriction := range []string{
		`<xs:length value="4"/>`,
		`<xs:maxLength value="2"/>`,
	} {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:simpleType name="base"><xs:restriction base="badList">
    <xs:length value="3" fixed="true"/>
  </xs:restriction></xs:simpleType>
  <xs:simpleType name="derived"><xs:restriction base="base">`+restriction+`</xs:restriction></xs:simpleType>
</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaFacet)
	}
}

func TestUnavailableSimpleContentRestrictionsRetainDecidableFacetState(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:complexType name="badContent">
    <xs:simpleContent><xs:extension base="badList"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="base">
    <xs:simpleContent><xs:restriction base="badContent"><xs:length value="3" fixed="true"/></xs:restriction></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:simpleContent><xs:restriction base="base"><xs:length value="4"/></xs:restriction></xs:simpleContent>
  </xs:complexType>
</xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaFacet)
}

func TestUnavailableRestrictionsGroupSiblingPatternsByDerivationStep(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:simpleType name="restricted"><xs:restriction base="badList">
    <xs:pattern value="red"/><xs:pattern value="green"/>
  </xs:restriction></xs:simpleType>
  <xs:complexType name="badContent">
    <xs:simpleContent><xs:extension base="badList"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="restrictedContent">
    <xs:simpleContent><xs:restriction base="badContent">
      <xs:pattern value="red"/><xs:pattern value="green"/>
    </xs:restriction></xs:simpleContent>
  </xs:complexType>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	simpleID := simpleBuildTypeIDByName(t, build, "restricted")
	complexID := complexBuildTypeIDByName(t, build, "restrictedContent")
	for name, id := range map[string]runtime.SimpleTypeID{
		"simple restriction":        simpleID,
		"simpleContent restriction": build.ComplexTypes[complexID].TextType,
	} {
		shape := runtime.SimpleFastPathValidationForSimpleType(build.SimpleTypes[id])
		if shape.PatternGroupSize != 1 {
			t.Errorf("%s pattern derivation groups = %d, want 1", name, shape.PatternGroupSize)
		}
	}
}

func TestUnavailableRestrictionStillValidatesFacetSyntax(t *testing.T) {
	for _, facet := range []string{
		`<xs:length value="1"/>`,
		`<xs:pattern value="["/>`,
		`<xs:enumeration/>`,
	} {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad"><xs:union memberTypes="absent xs:int"/></xs:simpleType>
  <xs:simpleType name="restricted"><xs:restriction base="bad">`+facet+`</xs:restriction></xs:simpleType>
</xs:schema>`))})
		if err == nil {
			t.Fatalf("Compile() accepted invalid unavailable-base facet %s", facet)
		}
	}
}

func TestMissingSchemaNamespaceTypesInvalidateSchema(t *testing.T) {
	for _, test := range []struct {
		name        string
		declaration string
	}{
		{name: "element", declaration: `<xs:element name="bad" type="xs:absent"/>`},
		{name: "attribute", declaration: `<xs:attribute name="bad" type="xs:absent"/>`},
		{name: "list", declaration: `<xs:simpleType name="bad"><xs:list itemType="xs:absent"/></xs:simpleType>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.declaration+`</xs:schema>`))})
			expectCode(t, err, xsderrors.CodeSchemaReference)
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || xerr.Path != "schema.xsd" || xerr.Line != 2 || xerr.Column == 0 {
				t.Fatalf("Compile() location = %#v, want schema.xsd:2 with nonzero column", err)
			}
		})
	}
}

func TestMissingListItemTypeInvalidatesAffectedElements(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:complexType name="badComplex"><xs:simpleContent><xs:extension base="badList"/></xs:simpleContent></xs:complexType>
  <xs:element name="bad" type="badList" nillable="true"/>
  <xs:element name="badComplex" type="badComplex"/>
  <xs:element name="good" type="xs:int"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<good>1</good>`)
	mustNotValidateRuntime(t, engine, `<bad>value</bad>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<bad xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<badComplex>value</badComplex>`, xsderrors.CodeValidationElement)
}

func TestUnavailableElementRecoverySkipsBrokenValuePath(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="bad" type="badList"/>
      <xs:element name="good" type="xs:int"/>
    </xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	session, err := validate.NewSession(engine, validate.Options{MaxErrors: 10})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	err = session.Validate(strings.NewReader(`<root><bad><ignored/></bad><good>1</good></root>`))
	if got, want := validationErrorCodes(err), []xsderrors.Code{xsderrors.CodeValidationElement}; !slices.Equal(got, want) {
		t.Fatalf("validation codes = %v, want %v; err=%v", got, want, err)
	}
}

func TestUnavailableElementTypeDefersValueConstraintAssessment(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:element name="direct" type="absent" default="value"/>
  <xs:element name="transitive" type="badList" fixed="value"/>
</xs:schema>`)
	mustNotValidateRuntime(t, engine, `<direct/>`, xsderrors.CodeValidationElement)
	mustNotValidateRuntime(t, engine, `<transitive>value</transitive>`, xsderrors.CodeValidationElement)
}

func TestUnavailableElementTypePrecedesXSIProcessingAndContainsRecovery(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad" type="absent" nillable="true"/>
</xs:schema>`)
	for _, doc := range []string{
		`<bad xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="invalid"><ignored/></bad>`,
		`<bad xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="unknown"><ignored/></bad>`,
		`<bad xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:string"><ignored/></bad>`,
	} {
		session, err := validate.NewSession(engine, validate.Options{MaxErrors: 10})
		if err != nil {
			t.Fatalf("NewSession() error = %v", err)
		}
		err = session.Validate(strings.NewReader(doc))
		if got, want := validationErrorCodes(err), []xsderrors.Code{xsderrors.CodeValidationElement}; !slices.Equal(got, want) {
			t.Fatalf("validation codes = %v, want %v; err=%v", got, want, err)
		}
	}
}

func TestUnavailableAttributeTypesReportValidationErrors(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="badList"><xs:list itemType="absent"/></xs:simpleType>
  <xs:attribute name="globalBad" type="badList" default="value"/>
  <xs:attribute name="directBad" type="absent" default="value"/>
  <xs:element name="local"><xs:complexType><xs:attribute name="bad" type="badList"/></xs:complexType></xs:element>
  <xs:element name="directLocal"><xs:complexType><xs:attribute name="bad" type="absent"/></xs:complexType></xs:element>
  <xs:element name="strict"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>
  <xs:element name="lax"><xs:complexType><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
  <xs:element name="skip"><xs:complexType><xs:anyAttribute processContents="skip"/></xs:complexType></xs:element>
</xs:schema>`)
	for _, doc := range []string{
		`<local bad="value"/>`,
		`<directLocal bad="value"/>`,
		`<strict globalBad="value"/>`,
		`<strict directBad="value"/>`,
		`<lax globalBad="value"/>`,
		`<lax directBad="value"/>`,
	} {
		mustNotValidateRuntime(t, engine, doc, xsderrors.CodeValidationAttribute)
	}
	mustValidateRuntime(t, engine, `<skip globalBad="value"/>`)
}

func TestElementDeclarationsMustBeConsistent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="a" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestExtendedElementDeclarationsMustBeConsistent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence><xs:element name="item" type="xs:int"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="bad">
    <xs:complexContent>
      <xs:extension base="base">
        <xs:sequence><xs:element name="item" type="xs:date"/></xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestTypeFinalBlocksDerivation(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="Base" final="extension"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="tns:Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Base" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="tns:Base"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base" final="extension">
    <xs:simpleContent>
      <xs:extension base="xs:string"><xs:attribute name="a"/></xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base"><xs:attribute name="b"/></xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestAnonymousSimpleTypeCannotHaveName(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="parent"><xs:restriction><xs:simpleType name="child"><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction></xs:simpleType></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
}

func TestSimpleDerivationAnnotationMustPrecedeContent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:list>
      <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
      <xs:annotation/>
    </xs:list>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:union>
      <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
      <xs:annotation/>
    </xs:union>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestSimpleContentRestrictionSimpleTypeMustPrecedeFacets(t *testing.T) {
	const prefix = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:simpleContent><xs:restriction base="Base">`
	const inline = `<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`
	const suffix = `</xs:restriction></xs:simpleContent></xs:complexType></xs:element></xs:schema>`

	if _, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("ordered.xsd", []byte(prefix+inline+`<xs:minLength value="1"/>`+suffix))}); err != nil {
		t.Fatalf("Compile(ordered) error = %v", err)
	}
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("reversed.xsd", []byte(prefix+`<xs:minLength value="1"/>`+inline+suffix))})
	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestTopLevelSimpleTypeRequiresName(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestRestrictionElementPropertiesCannotBeLoosened(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" fixed="foo" type="xs:string"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" fixed="bar" type="xs:string"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" block="extension restriction"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" block="extension substitution"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestRestrictionElementPreservesFixedValueIdentity(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="intValue"><xs:restriction base="xs:integer"/></xs:simpleType>
  <xs:complexType name="base">
    <xs:sequence><xs:element name="a" type="xs:decimal" fixed="5.0"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:element name="a" type="intValue" fixed="5"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
}

func TestRestrictionElementTypeCannotUseExtension(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test">
  <xs:complexType name="baseType"><xs:choice><xs:element name="f1"/><xs:element name="f2"/></xs:choice></xs:complexType>
  <xs:complexType name="extendedType">
    <xs:complexContent>
      <xs:extension base="t:baseType"><xs:choice><xs:element name="f3"/><xs:element name="f4"/></xs:choice></xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="base"><xs:choice><xs:element name="c1" type="t:baseType"/><xs:element name="c2"/></xs:choice></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="t:base"><xs:choice><xs:element name="c1" type="t:extendedType"/><xs:element name="c2"/></xs:choice></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRestrictionElementCanUseSubstitutionMember(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="c" substitutionGroup="d" type="xs:anyType"/>
  <xs:element name="d" type="xs:anyType"/>
  <xs:complexType name="base"><xs:sequence><xs:element ref="d"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element ref="c"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)
}

func TestSubstitutionMemberInheritsHeadType(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<member>1</member>`)
	mustNotValidateRuntime(t, engine, `<member>x</member>`, xsderrors.CodeValidationFacet)
}

func TestSubstitutionMemberWithMissingHeadUsesDefaultType(t *testing.T) {
	engine := mustCompileRuntime(t, `
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		  <xs:element name="member" substitutionGroup="missing"/>
		</xs:schema>`)
	mustValidateRuntime(t, engine, `<member>anything</member>`)
}

func TestSchemaAdmissionRejectsForeignElementsOutsideAnnotationPayload(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("foreign.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign">
  <f:extension/>
</xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestSchemaAdmissionTreatsAnnotationPayloadAsOpaque(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation id="outer"><xs:appinfo>
    <xs:element xs:unknown="allowed-in-payload" id="outer" name="not a schema name">
      <xs:future id="outer"/>
    </xs:element>
  </xs:appinfo></xs:annotation>
  <xs:element name="root"/>
</xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("opaque.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root/>`)
}

func TestSchemaAdmissionRejectsSchemaAttributesOnAnnotationEnvelopes(t *testing.T) {
	for _, envelope := range []string{"appinfo", "documentation"} {
		t.Run(envelope, func(t *testing.T) {
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:` + envelope + ` xs:bogus="value"/></xs:annotation></xs:schema>`
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes(envelope+".xsd", []byte(schema))})
			expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
		})
	}
}

func TestSchemaAdmissionRejectsDirectiveChildren(t *testing.T) {
	for _, directive := range []string{"include", "import"} {
		t.Run(directive, func(t *testing.T) {
			attrs := ` schemaLocation=""`
			if directive == "import" {
				attrs += ` namespace="urn:other"`
			}
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:` + directive + attrs + `><xs:element name="bad"/></xs:` + directive + `></xs:schema>`
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes(directive+".xsd", []byte(schema))})
			expectCode(t, err, xsderrors.CodeSchemaContentModel)
		})
	}
}

func TestSchemaCompileDiagnosticIdentifiesSource(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("good.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:good"/>`)),
		source.Bytes("broken.xsd", []byte("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n<xs:element/>\n</xs:schema>")),
	})
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || xerr.Path != "broken.xsd" || xerr.Line != 2 {
		t.Fatalf("Compile() error = %#v, want broken.xsd:2", err)
	}
}

func TestContentModelSubstitutionRespectsElementBlock(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head"/>
  <xs:element name="blocked" type="xs:int" block="substitution"/>
  <xs:element name="blockedMember" substitutionGroup="blocked"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element ref="head"/>
        <xs:element ref="blocked"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root><member>1</member></root>`)
	mustNotValidateRuntime(t, engine, `<root><blockedMember>1</blockedMember></root>`, xsderrors.CodeValidationElement)
}

func TestAnonymousLocalTypeCanRestrictContainingType(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns="urn:test">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="foo"/>
      <xs:element name="bar" minOccurs="0">
        <xs:complexType>
          <xs:complexContent>
            <xs:restriction base="base">
              <xs:sequence><xs:element name="foo"/></xs:sequence>
            </xs:restriction>
          </xs:complexContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="base"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:test"><foo/><bar><foo/></bar></t:root>`)
}

func TestNamedComplexTypeCannotDeriveFromItself(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="self">
    <xs:complexContent><xs:extension base="self"/></xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestComplexContentExtensionCannotDropMixedBase(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base" mixed="true">
    <xs:sequence><xs:element name="a"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base" mixed="true">
    <xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence>
  </xs:complexType>
  <xs:element name="r">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="base">
          <xs:sequence><xs:element name="b" minOccurs="0"/></xs:sequence>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRecursiveComplexTypeThroughElementRefCompiles(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="node"/>
  <xs:element name="child" type="node"/>
  <xs:complexType name="node">
    <xs:choice maxOccurs="unbounded">
      <xs:element ref="child" minOccurs="0"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestUnsupportedFeaturesAreExplicit(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:redefine schemaLocation="a.xsd"/></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeUnsupportedRedefine)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:anyAttribute notQName="##defined"/></xs:complexType></xs:element></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeUnsupportedXSD11)
}

func TestCompileOptionsSchemaXMLLimits(t *testing.T) {
	deepSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>ok</xs:documentation></xs:annotation></xs:schema>`
	_, err := compile.Compile(compile.Options{MaxSchemaDepth: 2}, []source.Source{source.Bytes("schema.xsd", []byte(deepSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
	if _, boundaryErr := compile.Compile(compile.Options{MaxSchemaDepth: 3}, []source.Source{source.Bytes("schema.xsd", []byte(deepSchema))}); boundaryErr != nil {
		t.Fatalf("Compile() depth boundary error = %v", boundaryErr)
	}

	attrSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:element name="root"/></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxSchemaAttributes: 1}, []source.Source{source.Bytes("schema.xsd", []byte(attrSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
	if _, boundaryErr := compile.Compile(compile.Options{MaxSchemaAttributes: 2}, []source.Source{source.Bytes("schema.xsd", []byte(attrSchema))}); boundaryErr != nil {
		t.Fatalf("Compile() attribute boundary error = %v", boundaryErr)
	}

	textSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` + strings.Repeat("x", 129) + `</xs:documentation></xs:annotation></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxSchemaTokenBytes: 128}, []source.Source{source.Bytes("schema.xsd", []byte(textSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)

	nodeSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo><payload/></xs:appinfo></xs:annotation></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxSchemaInstantiatedNodes: 3}, []source.Source{source.Bytes("schema.xsd", []byte(nodeSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
}

func TestSchemaParserDoesNotRetainOpaqueAnnotationPayload(t *testing.T) {
	limits, err := compile.NormalizeOptions(compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	root, err := compile.ParseSchemaRootForTest([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo>text<payload attr="value"><nested/></payload></xs:appinfo></xs:annotation></xs:schema>`), limits)
	if err != nil {
		t.Fatal(err)
	}
	appinfo := root.Children[0].Children[0]
	if appinfo.Text != "" || len(appinfo.Children) != 0 {
		t.Fatalf("retained opaque payload: text=%q children=%d", appinfo.Text, len(appinfo.Children))
	}
}

func TestCompileOptionsSchemaSourceByteLimit(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	if _, err := compile.Compile(compile.Options{MaxSchemaSourceBytes: int64(len(schema))}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() source byte boundary error = %v", err)
	}

	_, err := compile.Compile(compile.Options{MaxSchemaSourceBytes: int64(len(schema) - 1)}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func TestSchemaNamespaceContextsAreIsolated(t *testing.T) {
	limits, err := compile.NormalizeOptions(compile.Options{})
	if err != nil {
		t.Fatalf("NormalizeOptions() error = %v", err)
	}
	root, err := compile.ParseSchemaRootForTest([]byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:test"
           xmlns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="base"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:annotation xmlns:t="urn:other" xmlns="">
    <xs:documentation>namespace reset must stay local</xs:documentation>
  </xs:annotation>
  <xs:element name="prefixed" type="t:base"/>
  <xs:element name="defaulted" type="base"/>
  <xs:element name="local" xmlns:u="urn:test" type="u:base"/>
</xs:schema>`), limits)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if got := root.NS["t"]; got != "urn:test" {
		t.Fatalf("root prefix t = %q, want urn:test", got)
	}
	if got := root.NS[""]; got != "urn:test" {
		t.Fatalf("root default namespace = %q, want urn:test", got)
	}
	annotation := root.Children[1]
	if got := annotation.NS["t"]; got != "urn:other" {
		t.Fatalf("annotation prefix t = %q, want urn:other", got)
	}
	if got := annotation.NS[""]; got != "" {
		t.Fatalf("annotation default namespace = %q, want empty", got)
	}
	prefixed := root.Children[2]
	if got := prefixed.NS["t"]; got != "urn:test" {
		t.Fatalf("sibling prefix t = %q, want urn:test", got)
	}
	defaulted := root.Children[3]
	if got := defaulted.NS[""]; got != "urn:test" {
		t.Fatalf("sibling default namespace = %q, want urn:test", got)
	}
	local := root.Children[4]
	if got := local.NS["u"]; got != "urn:test" {
		t.Fatalf("local prefix u = %q, want urn:test", got)
	}
}

func TestCompileOptionsRejectNegativeLimits(t *testing.T) {
	schemaSource := source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`))
	tests := []compile.Options{
		{MaxSchemaDepth: -1},
		{MaxSchemaAttributes: -1},
		{MaxSchemaTokenBytes: -1},
		{MaxSchemaSourceBytes: -1},
		{MaxSchemaNames: -1},
		{MaxContentModelStates: -1},
	}
	for _, opts := range tests {
		_, err := compile.Compile(opts, []source.Source{schemaSource})
		expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	}
}

func TestFreezeReplaysResolvedQNameValueConstraint(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(build); err != nil {
		t.Fatalf("validateSchemaBuild() error = %v", err)
	}
}

func TestRuntimeKeyRefAmbiguousSiblingKeysWithSameDisplayPathDoesNotResolve(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="group" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string" use="required"/>
          </xs:complexType>
          <xs:key name="groupKey">
            <xs:selector xpath="."/>
            <xs:field xpath="@id"/>
          </xs:key>
        </xs:element>
      </xs:sequence>
      <xs:attribute name="rid" type="xs:string" use="required"/>
    </xs:complexType>
    <xs:keyref name="rootRef" refer="groupKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@rid"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	mustNotValidateRuntime(t, engine, `<root rid="1"><group id="1"/><group id="1"/></root>`, xsderrors.CodeValidationIdentity)
}

func TestFreezeRuntimeConsumesCompilerRuntime(t *testing.T) {
	c, rt := frozenCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="code" type="Code" use="required" fixed="US"/>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="child"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`)
	engine := rt
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)

	if !reflect.ValueOf(*c.RuntimeForTest()).IsZero() {
		t.Fatal("freezeRuntime did not clear compile.Compiler runtime")
	}
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)
}

func TestFreezeRuntimeClearsCompilerMutationAliases(t *testing.T) {
	c := compiledCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="code" type="Code" use="required" fixed="US"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	frozen, err := compile.FreezeCompilerRuntimeForTest(c)
	if err != nil {
		t.Fatalf("compile.FreezeCompilerRuntimeForTest() error = %v", err)
	}
	rt := validationRuntimeForTest(t, frozen)
	engine := rt

	if !reflect.ValueOf(*c.RuntimeForTest()).IsZero() {
		t.Fatal("freezeRuntime did not clear compile.Compiler runtime")
	}
	if !c.NameInternerIsZeroForTest() {
		t.Fatal("freezeRuntime did not clear compile.Compiler name interner")
	}
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)
}

func TestPublishedSchemaOwnsValidationStorage(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:complexType name="Item">
    <xs:attribute name="id" type="xs:string" use="required"/>
    <xs:attribute name="code" type="Code" use="required" fixed="US"/>
  </xs:complexType>
  <xs:element name="head" type="Item" abstract="true"/>
  <xs:element name="item" type="Item" substitutionGroup="head"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:choice maxOccurs="unbounded">
        <xs:element ref="head"/>
        <xs:element name="a"/>
        <xs:element name="b"/>
        <xs:element name="c"/>
        <xs:element name="d"/>
        <xs:element name="e"/>
        <xs:element name="f"/>
        <xs:element name="g"/>
        <xs:element name="h"/>
      </xs:choice>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	c := compiledCompilerRuntime(t, schema)
	build := c.RuntimeForTest()
	aliases := *build

	rootName := mustQName(t, &aliases.Names, "root")
	headName := mustQName(t, &aliases.Names, "head")
	itemName := mustQName(t, &aliases.Names, "item")
	codeName := mustQName(t, &aliases.Names, "code")
	rootID := aliases.GlobalElements[rootName]
	headID := aliases.GlobalElements[headName]
	codeID := simpleBuildTypeIDByName(t, &aliases, "Code")
	itemType := complexBuildTypeIDByName(t, &aliases, "Item")
	attrSetID := aliases.ComplexTypes[itemType].Attrs
	rootType, ok := aliases.Elements[rootID].Type.Complex()
	if !ok {
		t.Fatal("root element type is not complex")
	}
	modelID := aliases.ComplexTypes[rootType].Content
	identityID := aliases.Elements[rootID].Identity[0]

	published, err := runtime.PublishSchema(build)
	if err != nil {
		t.Fatalf("runtime.PublishSchema() error = %v", err)
	}

	aliases.GlobalElements[rootName] = headID
	aliases.GlobalTypes[mustQName(t, &aliases.Names, "Code")] = runtime.ComplexRef(itemType)
	aliases.SimpleTypes[codeID].Union[0] = runtime.NoSimpleType
	aliases.Elements[rootID].Identity[0] = runtime.NoIdentityConstraint
	aliases.Identities[identityID].Selector[0].Steps[0].Name = headName
	aliases.Identities[identityID].Fields[0].Paths[0].Attribute = itemName

	attrs := &aliases.AttributeUseSets[attrSetID]
	codeSlot := attrs.Index[codeName]
	attrs.Index[codeName] = 0
	attrs.Uses[codeSlot].Fixed.Canonical = "CA"
	attrs.Required[0] = codeSlot
	attrs.ValueConstraints[0] = 0

	for rowIndex := range aliases.CompiledModels[modelID].Rows {
		row := &aliases.CompiledModels[modelID].Rows[rowIndex]
		for name := range row.Index.NameToEdge {
			row.Index.NameToEdge[name] = 0
		}
		for i := range row.Index.WildcardEdges {
			row.Index.WildcardEdges[i] = 0
		}
	}
	interner := runtime.NewNameInterner(&aliases.Names)
	if _, err := interner.InternQName("urn:poison", "poison"); err != nil {
		t.Fatalf("InternQName() error = %v", err)
	}

	engine := validationRuntimeForTest(t, published)
	if _, ok := engine.LookupQName("urn:poison", "poison"); ok {
		t.Fatal("published name table retained compiler storage")
	}
	if member, ok := aliases.Substitutions.MemberByName(headID, itemName); !ok || member == runtime.NoElement {
		t.Fatal("test schema did not compile a substitution lookup")
	}
	mustValidateRuntime(t, engine, `<root><item id="one" code="US"/><h/></root>`)
	mustNotValidateRuntime(t, engine, `<root><item id="one" code="CA"/></root>`, xsderrors.CodeValidationAttribute)
	mustNotValidateRuntime(t, engine, `<root><item id="same" code="US"/><item id="same" code="US"/></root>`, xsderrors.CodeValidationIdentity)
}

func TestFreezeRuntimeKeepsCompilerStateOnValidationFailure(t *testing.T) {
	c := compiledCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="r" type="xs:string"/>
</xs:schema>`)
	build := c.RuntimeForTest()
	rootName := mustQName(t, &build.Names, "r")
	build.GlobalElements[rootName] = runtime.NoElement

	_, err := compile.FreezeCompilerRuntimeForTest(c)
	if err == nil {
		t.Fatal("compile.FreezeCompilerRuntimeForTest() error = nil, want validation error")
	}
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if reflect.ValueOf(*c.RuntimeForTest()).IsZero() {
		t.Fatal("FreezeCompilerRuntimeForTest cleared compile.Compiler runtime after validation failure")
	}
	if c.NameInternerIsZeroForTest() {
		t.Fatal("FreezeCompilerRuntimeForTest cleared compile.Compiler name interner after validation failure")
	}
}

func frozenCompilerRuntime(t *testing.T, schema string) (*compile.Compiler, *runtime.Schema) {
	t.Helper()
	c := compiledCompilerRuntime(t, schema)
	frozen, err := compile.FreezeCompilerRuntimeForTest(c)
	if err != nil {
		t.Fatalf("compile.FreezeCompilerRuntimeForTest() error = %v", err)
	}
	return c, validationRuntimeForTest(t, frozen)
}

func validationRuntimeForTest(tb testing.TB, rt *runtime.Schema) *runtime.Schema {
	tb.Helper()
	if rt != nil {
		return rt
	}
	tb.Fatal("frozen runtime view is nil")
	return nil
}

func compiledCompilerRuntime(t *testing.T, schema string) *compile.Compiler {
	t.Helper()
	limits, err := compile.NormalizeOptions(compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := compile.NewCompilerForTest(limits)
	if err != nil {
		t.Fatal(err)
	}
	err = c.LoadForTest([]source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	err = c.IndexForTest()
	if err != nil {
		t.Fatalf("index() error = %v", err)
	}
	err = c.CompileGlobalsForTest()
	if err != nil {
		t.Fatalf("compileGlobals() error = %v", err)
	}
	return c
}

func TestCompiledSimpleFastPathDerivedFromFacets(t *testing.T) {
	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt"><xs:restriction base="xs:int"/></xs:simpleType>
  <xs:simpleType name="MyShort"><xs:restriction base="xs:short"/></xs:simpleType>
  <xs:simpleType name="TightInt"><xs:restriction base="xs:int"><xs:maxInclusive value="10"/></xs:restriction></xs:simpleType>
</xs:schema>`)

	if got := build.SimpleTypes[build.Builtin.Int].Fast; got != runtime.SimpleFastInt {
		t.Fatalf("xs:int Fast = %v, want runtime.SimpleFastInt", got)
	}
	if got := build.SimpleTypes[simpleBuildTypeIDByName(t, build, "MyInt")].Fast; got != runtime.SimpleFastInt {
		t.Fatalf("MyInt Fast = %v, want runtime.SimpleFastInt", got)
	}
	if got := build.SimpleTypes[simpleBuildTypeIDByName(t, build, "MyShort")].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("MyShort Fast = %v, want runtime.SimpleFastNone", got)
	}
	if got := build.SimpleTypes[simpleBuildTypeIDByName(t, build, "TightInt")].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("TightInt Fast = %v, want runtime.SimpleFastNone", got)
	}
}

func TestSimpleContentFacetRestrictionRecomputesFastPath(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:int"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root">
    <xs:complexType>
      <xs:simpleContent>
        <xs:restriction base="Base">
          <xs:maxInclusive value="10"/>
        </xs:restriction>
      </xs:simpleContent>
    </xs:complexType>
	</xs:element>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	root := build.GlobalElements[mustQName(t, &build.Names, "root")]
	complexID, ok := build.Elements[root].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	textType := build.ComplexTypes[complexID].TextType
	if got := build.SimpleTypes[textType].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("simple content text type Fast = %v, want runtime.SimpleFastNone", got)
	}
	engine := mustCompileRuntime(t, schema)
	mustValidateRuntime(t, engine, `<root>10</root>`)
	mustNotValidateRuntime(t, engine, `<root>11</root>`, xsderrors.CodeValidationFacet)
}

func TestSimpleContentRestrictionAllowsEmptiableMixedBaseWithInlineType(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Mixed" mixed="true"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:simpleContent>
        <xs:restriction base="Mixed">
          <xs:simpleType>
            <xs:restriction base="xs:string"/>
          </xs:simpleType>
        </xs:restriction>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	mustValidateRuntime(t, engine, `<root>value</root>`)
}

type qnameLookup interface {
	LookupQName(namespace, local string) (runtime.QName, bool)
}

type compileCloseErrorReader struct {
	io.Reader
	err error
}

func writeCompileTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r compileCloseErrorReader) Close() error { return r.err }

func mustQName(t *testing.T, rt qnameLookup, local string) runtime.QName {
	t.Helper()
	q, ok := rt.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	return q
}

func TestFixedWhitespaceFacetFreezes(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Collapsed">
    <xs:restriction base="xs:string">
      <xs:whiteSpace value="collapse" fixed="true"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Collapsed"/>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(build); err != nil {
		t.Fatalf("validateSchemaBuild() error = %v", err)
	}
}

func TestMixedSimpleContentExtensionChain(t *testing.T) {
	const mixedBase = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A" mixed="true">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:complexContent mixed="true"><xs:extension base="A"/></xs:complexContent>
  </xs:complexType>
  <xs:complexType name="C">
    <xs:complexContent mixed="true"><xs:extension base="B"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="C"/>
</xs:schema>`
	mustCompileRuntime(t, mixedBase)

	const nonMixedBase = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:complexContent mixed="true"><xs:extension base="A"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="B"/>
</xs:schema>`
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(nonMixedBase))})
	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRuntimeElementAccessor(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	rt := publishedRuntime(t, engine)
	if _, ok := rt.Element(runtime.NoElement); ok {
		t.Error("element(noElement) resolved, want miss")
	}
	if _, ok := rt.Element(runtime.ElementID(1 << 30)); ok {
		t.Error("element(out of range) resolved, want miss")
	}
	rootName := mustQName(t, rt, "root")
	_, rootInfo, ok := rt.RootElement(runtime.RuntimeName{Known: true, Name: rootName})
	if !ok || !rootInfo.Type.IsSimple() {
		t.Fatal("root element is missing")
	}
}
