package compiler

import (
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func mustPrepared(t *testing.T, schemaXML string) *Prepared {
	t.Helper()
	prepared, err := Prepare(mustParsedSchema(t, schemaXML))
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	return prepared
}

func mustParsedSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return sch
}

func TestPrepareValidatorsRejectsNilInputs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	var nilPrepared *Prepared
	if _, err := nilPrepared.Build(BuildConfig{}); err == nil {
		t.Fatal("nil prepared Build() expected error")
	}

	prepared := mustPrepared(t, schemaXML)
	if prepared.build.validators != nil {
		t.Fatal("validators should be lazy before Build()")
	}
	if _, err := prepared.Build(BuildConfig{}); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if prepared.build.validators == nil {
		t.Fatal("Build() did not populate cached validators")
	}

	broken := &Prepared{
		registry:     prepared.registry,
		refs:         prepared.refs,
		complexTypes: prepared.complexTypes,
	}
	if _, err := broken.Build(BuildConfig{}); err == nil {
		t.Fatal("Build(nil schema) expected error")
	}
	broken = &Prepared{
		schema:       prepared.schema,
		refs:         prepared.refs,
		complexTypes: prepared.complexTypes,
	}
	if _, err := broken.Build(BuildConfig{}); err == nil {
		t.Fatal("Build(nil registry) expected error")
	}
	broken = &Prepared{
		schema:       prepared.schema,
		registry:     prepared.registry,
		complexTypes: prepared.complexTypes,
	}
	if _, err := broken.Build(BuildConfig{}); err == nil {
		t.Fatal("Build(nil references) expected error")
	}
}

func TestPreparedBuildCachesValidators(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	prepared := mustPrepared(t, schemaXML)

	rtFirst, err := prepared.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("first Build() error = %v", err)
	}
	cachedValidators := prepared.build.validators
	if cachedValidators == nil {
		t.Fatal("Build() did not cache validators")
	}

	rtSecond, err := prepared.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
	if prepared.build.validators != cachedValidators {
		t.Fatal("Build() replaced cached validators")
	}
	if rtFirst.BuildHash != rtSecond.BuildHash {
		t.Fatalf("build hash mismatch: first=%x second=%x", rtFirst.BuildHash, rtSecond.BuildHash)
	}
	if len(rtFirst.Types) != len(rtSecond.Types) {
		t.Fatalf("type count mismatch: first=%d second=%d", len(rtFirst.Types), len(rtSecond.Types))
	}
}

func TestPreparedBuildConcurrent(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	prepared := mustPrepared(t, schemaXML)

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Go(func() {
			_, err := prepared.Build(BuildConfig{MaxOccursLimit: 1024})
			errs <- err
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("prepared.Build() concurrent error = %v", err)
		}
	}
}

func TestPreparedBuildSimpleContentRestriction(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:sc"
           xmlns:tns="urn:sc"
           elementFormDefault="qualified">
  <xs:complexType name="MeasureType">
    <xs:simpleContent>
      <xs:extension base="xs:double"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="LengthType">
    <xs:simpleContent>
      <xs:restriction base="tns:MeasureType"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:LengthType"/>
</xs:schema>`

	preparedState := mustPrepared(t, schemaXML)
	rtFirst, err := preparedState.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("first Build() error = %v", err)
	}
	cachedValidators := preparedState.build.validators
	if cachedValidators == nil {
		t.Fatal("Build() did not cache validators")
	}
	rtSecond, err := preparedState.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
	if preparedState.build.validators != cachedValidators {
		t.Fatal("Build() replaced cached validators")
	}
	if len(rtFirst.Elements) != len(rtSecond.Elements) {
		t.Fatalf("element count mismatch: first=%d second=%d", len(rtFirst.Elements), len(rtSecond.Elements))
	}
	rootFirst := rtFirst.Elements[1]
	rootSecond := rtSecond.Elements[1]
	if rootFirst.Type != rootSecond.Type {
		t.Fatalf("root type mismatch: first=%d second=%d", rootFirst.Type, rootSecond.Type)
	}
	typFirst := rtFirst.Types[rootFirst.Type]
	typSecond := rtSecond.Types[rootSecond.Type]
	if typFirst.Validator != typSecond.Validator {
		t.Fatalf("root validator mismatch: first=%d second=%d", typFirst.Validator, typSecond.Validator)
	}
	ctFirst := rtFirst.ComplexTypes[typFirst.Complex.ID]
	ctSecond := rtSecond.ComplexTypes[typSecond.Complex.ID]
	if ctFirst.TextValidator != ctSecond.TextValidator {
		t.Fatalf("simple-content text validator mismatch: first=%d second=%d", ctFirst.TextValidator, ctSecond.TextValidator)
	}
}
