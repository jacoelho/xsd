package validationengine_test

import (
	"cmp"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/set"
	"github.com/jacoelho/xsd/internal/validationengine"
)

func TestEngineParityWithPublicAPI(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)},
	}

	schema, err := xsd.LoadWithOptions(fsys, "schema.xsd", xsd.NewLoadOptions())
	if err != nil {
		t.Fatalf("load public schema: %v", err)
	}
	prepared, err := set.Prepare(set.PrepareConfig{
		FS:       fsys,
		Location: "schema.xsd",
	})
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	engine := validationengine.NewEngine(rt)

	valid := `<root xmlns="urn:test"><a>ok</a></root>`
	if err := engine.Validate(strings.NewReader(valid)); err != nil {
		t.Fatalf("engine valid doc: %v", err)
	}
	if err := schema.Validate(strings.NewReader(valid)); err != nil {
		t.Fatalf("public valid doc: %v", err)
	}

	invalid := `<root xmlns="urn:test"><b>bad</b></root>`
	engineCodes := sortedValidationCodes(engine.Validate(strings.NewReader(invalid)))
	publicCodes := sortedValidationCodes(schema.Validate(strings.NewReader(invalid)))
	if !slices.Equal(engineCodes, publicCodes) {
		t.Fatalf("validation code mismatch: engine=%v public=%v", engineCodes, publicCodes)
	}
}

func TestEngineConcurrentValidation(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)},
	}
	prepared, err := set.Prepare(set.PrepareConfig{
		FS:       fsys,
		Location: "schema.xsd",
	})
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	engine := validationengine.NewEngine(rt)

	doc := `<root xmlns="urn:test"><item>1</item><item>2</item><item>3</item></root>`
	const goroutines = 8
	const iterations = 30
	errCh := make(chan error, goroutines*iterations)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent validate error: %v", err)
	}
}

func BenchmarkEngineValidate(b *testing.B) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)},
	}
	prepared, err := set.Prepare(set.PrepareConfig{
		FS:       fsys,
		Location: "schema.xsd",
	})
	if err != nil {
		b.Fatalf("prepare schema: %v", err)
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		b.Fatalf("build runtime: %v", err)
	}
	engine := validationengine.NewEngine(rt)
	doc := `<root xmlns="urn:bench"><item>1</item><item>2</item><item>3</item></root>`

	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatalf("validate: %v", err)
		}
	}
}

func sortedValidationCodes(err error) []string {
	if err == nil {
		return nil
	}
	violations, ok := xsderrors.AsValidations(err)
	if !ok {
		return []string{"ERR:" + err.Error()}
	}
	codes := make([]string, 0, len(violations))
	for _, violation := range violations {
		codes = append(codes, violation.Code)
	}
	slices.SortStableFunc(codes, cmp.Compare[string])
	return codes
}
