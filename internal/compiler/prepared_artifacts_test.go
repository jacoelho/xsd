package compiler

import (
	"sync"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func mustPreparedArtifacts(t *testing.T, schemaXML string) (*PreparedArtifacts, *analysis.Registry, *analysis.ResolvedReferences) {
	t.Helper()
	sch := mustResolveSchema(t, schemaXML)
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}
	validators := mustCompiledValidators(t, sch, reg)
	prepared, err := PrepareBuildArtifacts(sch, reg, refs, validators)
	if err != nil {
		t.Fatalf("PrepareBuildArtifacts() error = %v", err)
	}
	return prepared, reg, refs
}

func mustCompiledValidators(t *testing.T, sch *parser.Schema, reg *analysis.Registry) *semantics.CompiledValidators {
	t.Helper()
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}
	sem, err := semantics.Build(sch, reg, refs)
	if err != nil {
		t.Fatalf("semantics.Build() error = %v", err)
	}
	if err := sem.Particles().ValidateUPA(); err != nil {
		t.Fatalf("Particles().ValidateUPA() error = %v", err)
	}
	validators, err := sem.CompiledValidators()
	if err != nil {
		t.Fatalf("CompiledValidators() error = %v", err)
	}
	return validators
}

func TestPrepareBuildArtifactsRejectsNilInputs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	sch := mustResolveSchema(t, schemaXML)
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}

	validators := mustCompiledValidators(t, sch, reg)

	if _, err := PrepareBuildArtifacts(nil, reg, refs, validators); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil schema) expected error")
	}
	if _, err := PrepareBuildArtifacts(sch, nil, refs, validators); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil registry) expected error")
	}
	if _, err := PrepareBuildArtifacts(sch, reg, nil, validators); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil refs) expected error")
	}
	if _, err := PrepareBuildArtifacts(sch, reg, refs, nil); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil validators) expected error")
	}
}

func TestPreparedArtifactsBuildMatchesDirectBuild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	prepared, reg, refs := mustPreparedArtifacts(t, schemaXML)

	rtPrepared, err := prepared.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("prepared.Build() error = %v", err)
	}
	rtDirect, err := BuildArtifacts(prepared.Schema(), reg, refs, prepared.Validators(), BuildConfig{})
	if err != nil {
		t.Fatalf("BuildArtifacts() error = %v", err)
	}

	if rtPrepared.BuildHash != rtDirect.BuildHash {
		t.Fatalf("build hash mismatch: prepared=%x direct=%x", rtPrepared.BuildHash, rtDirect.BuildHash)
	}
	if len(rtPrepared.Types) != len(rtDirect.Types) {
		t.Fatalf("type count mismatch: prepared=%d direct=%d", len(rtPrepared.Types), len(rtDirect.Types))
	}
}

func TestPreparedArtifactsBuildConcurrent(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	prepared, _, _ := mustPreparedArtifacts(t, schemaXML)

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

func TestPrepareBuildArtifactsWithPrecomputedValidatorsSimpleContentRestriction(t *testing.T) {
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

	sch := mustResolveSchema(t, schemaXML)
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}
	sem, err := semantics.Build(sch, reg, refs)
	if err != nil {
		t.Fatalf("semantics.Build() error = %v", err)
	}
	if err := sem.Particles().ValidateUPA(); err != nil {
		t.Fatalf("Particles().ValidateUPA() error = %v", err)
	}
	validators, err := sem.CompiledValidators()
	if err != nil {
		t.Fatalf("CompiledValidators() error = %v", err)
	}
	prepared, err := PrepareBuildArtifacts(sch, reg, refs, validators)
	if err != nil {
		t.Fatalf("PrepareBuildArtifacts() error = %v", err)
	}
	if _, err := prepared.Build(BuildConfig{}); err != nil {
		t.Fatalf("prepared.Build() error = %v", err)
	}
}
