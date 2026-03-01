package validator_test

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/set"
	"github.com/jacoelho/xsd/internal/validator"
)

func TestEngineValidateNilSchema(t *testing.T) {
	engine := validator.NewEngine(nil)
	err := engine.Validate(strings.NewReader(`<root/>`))
	validations, ok := xsderrors.AsValidations(err)
	if !ok || len(validations) == 0 {
		t.Fatalf("Validate() errors = %v, want schema-not-loaded validation", err)
	}
	if got, want := validations[0].Code, string(xsderrors.ErrSchemaNotLoaded); got != want {
		t.Fatalf("Validate() code = %q, want %q", got, want)
	}
}

func TestEngineSessionParity(t *testing.T) {
	rt := buildRuntimeForTest(t)
	engine := validator.NewEngine(rt)
	session := validator.NewSession(rt)
	doc := `<root xmlns="urn:test"><child>bad</child></root>`

	engineCodes := sortedValidationCodes(engine.Validate(strings.NewReader(doc)))
	sessionCodes := sortedValidationCodes(session.Validate(strings.NewReader(doc)))
	if !slices.Equal(engineCodes, sessionCodes) {
		t.Fatalf("validation code mismatch: engine=%v session=%v", engineCodes, sessionCodes)
	}
}

func TestEngineConcurrentValidation(t *testing.T) {
	rt := buildConcurrentRuntimeForTest(t)
	engine := validator.NewEngine(rt)

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

func buildRuntimeForTest(t *testing.T) *runtime.Schema {
	t.Helper()
	prepared, err := set.Prepare(set.PrepareConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:test"
	xmlns:tns="urn:test"
	elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
		},
		Location: "schema.xsd",
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	return rt
}

func buildConcurrentRuntimeForTest(t *testing.T) *runtime.Schema {
	t.Helper()
	prepared, err := set.Prepare(set.PrepareConfig{
		FS: fstest.MapFS{
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
		},
		Location: "schema.xsd",
	})
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	return rt
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
	slices.Sort(codes)
	return codes
}
