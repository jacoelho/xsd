package xsd_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
)

const publicAPISchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:int"/></xs:schema>`

type readmeResolver map[string]string

func (r readmeResolver) ResolveSchema(_ string, location string) (xsd.SchemaSource, error) {
	data, ok := r[location]
	if !ok {
		return xsd.SchemaSource{}, xsd.ErrSchemaNotFound
	}
	return xsd.Reader(location, strings.NewReader(data)), nil
}

func ExampleCompile() {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(publicAPISchema)))
	if err != nil {
		fmt.Println(err)
		return
	}
	err = engine.Validate(strings.NewReader(`<root>7</root>`))
	fmt.Println("valid:", err == nil)
	// Output: valid: true
}

func ExampleCompileWithOptions() {
	engine, err := xsd.CompileWithOptions(
		xsd.CompileOptions{
			MaxSchemaDepth:      256,
			MaxSchemaAttributes: 256,
			MaxSchemaTokenBytes: 4 << 20,
			MaxSchemaNames:      0,
			MaxFiniteOccurs:     1_000_000,
		},
		xsd.Reader("schema.xsd", strings.NewReader(publicAPISchema)),
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = engine.Validate(strings.NewReader(`<root>7</root>`))
	fmt.Println("valid:", err == nil)
	// Output: valid: true
}

func ExampleReader() {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(publicAPISchema)))
	if err != nil {
		fmt.Println(err)
		return
	}
	err = engine.Validate(strings.NewReader(`<root>42</root>`))
	fmt.Println("valid:", err == nil)
	// Output: valid: true
}

func ExampleSchemaSource_WithResolver() {
	schema := strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Root"/>
</xs:schema>`)
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", schema).WithResolver(readmeResolver{
		"types.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Root"><xs:sequence/></xs:complexType>
</xs:schema>`,
	}))
	if err != nil {
		fmt.Println(err)
		return
	}
	err = engine.Validate(strings.NewReader(`<root/>`))
	fmt.Println("valid:", err == nil)
	// Output: valid: true
}

func ExampleError() {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(publicAPISchema)))
	if err != nil {
		fmt.Println(err)
		return
	}
	err = engine.Validate(strings.NewReader(`<root>x</root>`))
	if xerr, ok := errors.AsType[*xsd.Error](err); ok {
		fmt.Println(xerr.Category)
		fmt.Println(xerr.Code)
	}
	// Output:
	// validation
	// validation.facet
}

func ExampleEngine_Validate() {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(publicAPISchema)))
	if err != nil {
		fmt.Println(err)
		return
	}
	docs := []string{`<root>1</root>`, `<root>2</root>`, `<root>3</root>`}
	var wg sync.WaitGroup
	errs := make(chan error, len(docs))
	for _, doc := range docs {
		wg.Go(func() {
			errs <- engine.Validate(strings.NewReader(doc))
		})
	}
	wg.Wait()
	close(errs)
	valid := true
	for err := range errs {
		if err != nil {
			valid = false
		}
	}
	fmt.Println("valid:", valid)
	// Output: valid: true
}

func TestPublicErrorInspection(t *testing.T) {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(publicAPISchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	err = engine.Validate(strings.NewReader(`<root>x</root>`))
	if err == nil {
		t.Fatal("Validate() succeeded")
	}
	xerr, ok := errors.AsType[*xsd.Error](err)
	if !ok {
		t.Fatalf("Validate() error type = %T", err)
	}
	if xerr.Category != xsd.ValidationErrorCategory || xerr.Code != xsd.ErrValidationFacet {
		t.Fatalf("Validate() error = %s/%s", xerr.Category, xerr.Code)
	}
}

func TestFilePublicAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.xsd")
	if err := os.WriteFile(path, []byte(publicAPISchema), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := xsd.Compile(xsd.File(path))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root>1</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
