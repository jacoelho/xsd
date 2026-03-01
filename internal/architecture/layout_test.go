package architecture_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyPackagesRemoved(t *testing.T) {
	root := repoRoot(t)
	legacy := []string{
		"internal/pipeline",
		"internal/schemaanalysis",
		"internal/schemafacet",
		"internal/schemaprep",
		"internal/schemaxml",
		"internal/source",
		"internal/state",
		"internal/validationengine",
	}

	for _, rel := range legacy {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy package still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
}

func TestCorePhasePackagesHaveDoc(t *testing.T) {
	root := repoRoot(t)
	required := []string{
		"internal/preprocessor",
		"internal/parser",
		"internal/semanticresolve",
		"internal/semanticcheck",
		"internal/analysis",
		"internal/normalize",
		"internal/compiler",
		"internal/runtimeassemble",
		"internal/set",
	}

	for _, rel := range required {
		doc := filepath.Join(root, rel, "doc.go")
		if _, err := os.Stat(doc); err != nil {
			if os.IsNotExist(err) {
				t.Errorf("missing package doc: %s", doc)
				continue
			}
			t.Fatalf("stat %s: %v", doc, err)
		}
	}
}

func TestPublicMonolithFilesRemoved(t *testing.T) {
	root := repoRoot(t)
	legacy := []string{
		"xsd.go",
		"schemaset.go",
		"options.go",
	}

	for _, rel := range legacy {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy public monolith file still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
}

func TestPhaseMonolithFilesRemoved(t *testing.T) {
	root := repoRoot(t)
	legacy := []string{
		"internal/runtimeassemble/schema.go",
		"internal/preprocessor/schema_metadata.go",
		"internal/validator/runtime_validate_namespace.go",
		"internal/validator/runtime_validate_start.go",
		"internal/validator/runtime_start_flow.go",
		"internal/validator/runtime_start_element.go",
		"internal/validator/runtime_validate_end_text.go",
		"internal/validator/runtime_validate_end.go",
		"internal/validator/runtime_validate_session.go",
		"internal/validator/runtime_text.go",
		"internal/validator/runtime_error_types.go",
		"internal/validator/runtime_error_record.go",
		"internal/validator/runtime_model_core.go",
		"internal/validator/runtime_model_dispatch.go",
		"internal/validator/runtime_model_expected.go",
		"internal/validator/runtime_model_expected_sets.go",
		"internal/validator/runtime_attrs_complex.go",
		"internal/validator/runtime_errors.go",
		"internal/validator/value_atomic.go",
		"internal/validator/session_identity_lifecycle.go",
		"internal/validator/session_identity_finalize.go",
		"internal/validator/session_identity_flow.go",
		"internal/validator/session_identity_path_helpers.go",
		"internal/validator/session_identity_selectors.go",
		"internal/validator/session_identity_types.go",
		"internal/validator/session_identity_path_program.go",
		"internal/validator/session_types.go",
		"internal/validator/session_limits.go",
		"internal/validator/session_lifecycle.go",
		"internal/validator/runtime_validate_names.go",
		"internal/validator/runtime_name_intern.go",
		"internal/validator/runtime_attrs_classification.go",
		"internal/validator/runtime_attrs_validate.go",
		"internal/validator/runtime_value_types.go",
		"internal/validator/runtime_value_validate.go",
		"internal/validator/default_fixed_policy.go",
		"internal/validator/value_id_tracking.go",
		"internal/validator/validation_executor.go",
	}

	for _, rel := range legacy {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy phase monolith file still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
}
