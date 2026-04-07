package compiler

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestProcessDispatchesInOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	directives := []parser.Directive{
		{Kind: parser.DirectiveInclude, Include: parser.IncludeInfo{SchemaLocation: "a.xsd"}},
		{Kind: parser.DirectiveImport, Import: parser.ImportInfo{SchemaLocation: "b.xsd"}},
	}

	err := Process(
		directives,
		func(info parser.IncludeInfo) error {
			calls = append(calls, "include:"+info.SchemaLocation)
			return nil
		},
		func(info parser.ImportInfo) error {
			calls = append(calls, "import:"+info.SchemaLocation)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(calls) != 2 || calls[0] != "include:a.xsd" || calls[1] != "import:b.xsd" {
		t.Fatalf("calls = %#v, want ordered include/import dispatch", calls)
	}
}

func TestProcessImportRequiresSchemaLocation(t *testing.T) {
	t.Parallel()

	err := ProcessImport(ImportConfig[string]{})
	if err == nil || err.Error() != "import missing schemaLocation" {
		t.Fatalf("ProcessImport() error = %v, want missing-schemaLocation error", err)
	}
}

func TestProcessImportSkipsMissingLocationWhenAllowed(t *testing.T) {
	t.Parallel()

	called := false
	err := ProcessImport(ImportConfig[string]{
		AllowMissingLocation: true,
		Load: func(parser.ImportInfo) (LoadResult[string], error) {
			called = true
			return LoadResult[string]{}, nil
		},
	})
	if err != nil {
		t.Fatalf("ProcessImport() error = %v", err)
	}
	if called {
		t.Fatal("Load callback was called for missing schemaLocation")
	}
}

func TestProcessImportMergesLoadedSchema(t *testing.T) {
	t.Parallel()

	var merged struct {
		target string
		schema *parser.Schema
	}
	wantSchema := parser.NewSchema()

	err := ProcessImport(ImportConfig[string]{
		Info: parser.ImportInfo{SchemaLocation: "imp.xsd"},
		Load: func(info parser.ImportInfo) (LoadResult[string], error) {
			if info.SchemaLocation != "imp.xsd" {
				t.Fatalf("Load() schemaLocation = %q, want imp.xsd", info.SchemaLocation)
			}
			return LoadResult[string]{Schema: wantSchema, Target: "target", Status: StatusLoaded}, nil
		},
		Merge: func(schema *parser.Schema, target string) error {
			merged.target = target
			merged.schema = schema
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ProcessImport() error = %v", err)
	}
	if merged.target != "target" || merged.schema != wantSchema {
		t.Fatalf("merged = %+v, want loaded schema and target", merged)
	}
}

func TestProcessImportWrapsLoadError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	err := ProcessImport(ImportConfig[string]{
		Info: parser.ImportInfo{SchemaLocation: "imp.xsd"},
		Load: func(parser.ImportInfo) (LoadResult[string], error) {
			return LoadResult[string]{}, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ProcessImport() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestProcessIncludeSkippedMissingReturnsError(t *testing.T) {
	t.Parallel()

	err := ProcessInclude(IncludeConfig[string]{
		Info: parser.IncludeInfo{SchemaLocation: "inc.xsd"},
		Load: func(parser.IncludeInfo) (LoadResult[string], error) {
			return LoadResult[string]{Status: StatusSkippedMissing}, nil
		},
	})
	if err == nil || err.Error() != "included schema inc.xsd not found" {
		t.Fatalf("ProcessInclude() error = %v, want include-not-found error", err)
	}
}

func TestProcessIncludeMergesLoadedSchema(t *testing.T) {
	t.Parallel()

	var merged string
	wantSchema := parser.NewSchema()

	err := ProcessInclude(IncludeConfig[string]{
		Info: parser.IncludeInfo{SchemaLocation: "inc.xsd"},
		Load: func(parser.IncludeInfo) (LoadResult[string], error) {
			return LoadResult[string]{Schema: wantSchema, Target: "target", Status: StatusLoaded}, nil
		},
		Merge: func(schema *parser.Schema, target string) error {
			if schema != wantSchema {
				t.Fatalf("Merge() schema = %p, want %p", schema, wantSchema)
			}
			merged = target
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ProcessInclude() error = %v", err)
	}
	if merged != "target" {
		t.Fatalf("merged target = %q, want target", merged)
	}
}
