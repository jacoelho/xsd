package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/schemaxml"
)

func TestLoaderUsesConfiguredDocumentPool(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{
			Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:element name="root" type="xs:string"/></xs:schema>`),
		},
	}

	poolA := schemaxml.NewDocumentPool()
	poolB := schemaxml.NewDocumentPool()

	loaderA := NewLoader(Config{FS: fsys, DocumentPool: poolA})
	loaderB := NewLoader(Config{FS: fsys, DocumentPool: poolB})

	beforeA := poolA.Stats()
	beforeB := poolB.Stats()

	if _, err := loaderA.Load("schema.xsd"); err != nil {
		t.Fatalf("loaderA.Load() error = %v", err)
	}

	afterA := poolA.Stats()
	afterB := poolB.Stats()
	if afterA.Acquires <= beforeA.Acquires || afterA.Releases <= beforeA.Releases {
		t.Fatalf("poolA stats did not advance: before=%+v after=%+v", beforeA, afterA)
	}
	if afterB.Acquires != beforeB.Acquires || afterB.Releases != beforeB.Releases {
		t.Fatalf("poolB changed during loaderA.Load(): before=%+v after=%+v", beforeB, afterB)
	}

	if _, err := loaderB.Load("schema.xsd"); err != nil {
		t.Fatalf("loaderB.Load() error = %v", err)
	}

	finalA := poolA.Stats()
	finalB := poolB.Stats()
	if finalA.Acquires != afterA.Acquires || finalA.Releases != afterA.Releases {
		t.Fatalf("poolA changed during loaderB.Load(): afterA=%+v finalA=%+v", afterA, finalA)
	}
	if finalB.Acquires <= afterB.Acquires || finalB.Releases <= afterB.Releases {
		t.Fatalf("poolB stats did not advance: afterB=%+v finalB=%+v", afterB, finalB)
	}
}

func TestLoaderAcceptsZeroValueDocumentPool(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{
			Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`),
		},
	}

	pool := &schemaxml.DocumentPool{}
	loader := NewLoader(Config{FS: fsys, DocumentPool: pool})

	if _, err := loader.Load("schema.xsd"); err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}

	stats := pool.Stats()
	if stats.Acquires == 0 || stats.Releases == 0 {
		t.Fatalf("zero-value pool stats did not advance: %+v", stats)
	}
}
