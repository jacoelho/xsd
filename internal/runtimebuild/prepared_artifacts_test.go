package runtimebuild

import (
	"sync"
	"testing"

	schema "github.com/jacoelho/xsd/internal/semantic"
)

func mustPreparedArtifacts(t *testing.T, schemaXML string) (*PreparedArtifacts, *schema.Registry, *schema.ResolvedReferences) {
	t.Helper()
	sch := mustResolveSchema(t, schemaXML)
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	refs, err := schema.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}
	prepared, err := PrepareBuildArtifacts(sch, reg, refs)
	if err != nil {
		t.Fatalf("PrepareBuildArtifacts() error = %v", err)
	}
	return prepared, reg, refs
}

func TestPrepareBuildArtifactsRejectsNilInputs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	sch := mustResolveSchema(t, schemaXML)
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	refs, err := schema.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}

	if _, err := PrepareBuildArtifacts(nil, reg, refs); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil schema) expected error")
	}
	if _, err := PrepareBuildArtifacts(sch, nil, refs); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil registry) expected error")
	}
	if _, err := PrepareBuildArtifacts(sch, reg, nil); err == nil {
		t.Fatal("PrepareBuildArtifacts(nil refs) expected error")
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
	rtDirect, err := BuildArtifacts(prepared.schema, reg, refs, BuildConfig{})
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
