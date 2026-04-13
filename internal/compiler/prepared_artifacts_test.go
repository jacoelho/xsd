package compiler

import (
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/validatorbuild"
)

func mustPreparedValidators(t *testing.T, schemaXML string) (*Prepared, *analysis.Registry, *analysis.ResolvedReferences, *validatorbuild.ValidatorArtifacts) {
	t.Helper()
	prepared := mustPrepared(t, schemaXML)
	validators, err := prepared.ensureValidators()
	if err != nil {
		t.Fatalf("ensureValidators() error = %v", err)
	}
	return prepared, prepared.Registry(), prepared.References(), validators
}

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
	prepared := mustPrepared(t, schemaXML)
	validators, err := prepared.ensureValidators()
	if err != nil {
		t.Fatalf("ensureValidators() error = %v", err)
	}
	sch := prepared.Schema()
	reg := prepared.Registry()
	refs := prepared.References()

	if _, err := prepareValidators(nil, reg, refs, prepared.ComplexTypes()); err == nil {
		t.Fatal("prepareValidators(nil schema) expected error")
	}
	if _, err := prepareValidators(sch, nil, refs, prepared.ComplexTypes()); err == nil {
		t.Fatal("prepareValidators(nil registry) expected error")
	}
	if _, err := prepareValidators(sch, reg, nil, prepared.ComplexTypes()); err == nil {
		t.Fatal("prepareValidators(nil refs) expected error")
	}
	if _, err := Build(sch, reg, refs, nil, Config{}); err == nil {
		t.Fatal("Build(nil validators) expected error")
	}
	if validators == nil {
		t.Fatal("ensureValidators() returned nil validators")
	}
}

func TestPreparedBuildMatchesDirectBuild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	prepared, reg, refs, validators := mustPreparedValidators(t, schemaXML)

	rtPrepared, err := prepared.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("prepared.Build() error = %v", err)
	}
	rtDirect, err := Build(prepared.Schema(), reg, refs, validators, Config{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if rtPrepared.BuildHash != rtDirect.BuildHash {
		t.Fatalf("build hash mismatch: prepared=%x direct=%x", rtPrepared.BuildHash, rtDirect.BuildHash)
	}
	if len(rtPrepared.Types) != len(rtDirect.Types) {
		t.Fatalf("type count mismatch: prepared=%d direct=%d", len(rtPrepared.Types), len(rtDirect.Types))
	}
}

func TestPreparedBuildConcurrent(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	prepared, _, _, _ := mustPreparedValidators(t, schemaXML)

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

func TestPreparedBuildWithPrecomputedValidatorsSimpleContentRestriction(t *testing.T) {
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
	validators, err := preparedState.ensureValidators()
	if err != nil {
		t.Fatalf("ensureValidators() error = %v", err)
	}
	sch := preparedState.Schema()
	reg := preparedState.Registry()
	refs := preparedState.References()
	rtPrepared, err := preparedState.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("prepared.Build() error = %v", err)
	}
	rtDirect, err := Build(sch, reg, refs, validators, Config{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(rtPrepared.Elements) != len(rtDirect.Elements) {
		t.Fatalf("element count mismatch: prepared=%d direct=%d", len(rtPrepared.Elements), len(rtDirect.Elements))
	}
	rootPrepared := rtPrepared.Elements[1]
	rootDirect := rtDirect.Elements[1]
	if rootPrepared.Type != rootDirect.Type {
		t.Fatalf("root type mismatch: prepared=%d direct=%d", rootPrepared.Type, rootDirect.Type)
	}
	typPrepared := rtPrepared.Types[rootPrepared.Type]
	typDirect := rtDirect.Types[rootDirect.Type]
	if typPrepared.Validator != typDirect.Validator {
		t.Fatalf("root validator mismatch: prepared=%d direct=%d", typPrepared.Validator, typDirect.Validator)
	}
	ctPrepared := rtPrepared.ComplexTypes[typPrepared.Complex.ID]
	ctDirect := rtDirect.ComplexTypes[typDirect.Complex.ID]
	if ctPrepared.TextValidator != ctDirect.TextValidator {
		t.Fatalf("simple-content text validator mismatch: prepared=%d direct=%d", ctPrepared.TextValidator, ctDirect.TextValidator)
	}
}
