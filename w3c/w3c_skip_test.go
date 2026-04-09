package w3c

import (
	"errors"
	"testing"
)

func TestShouldSkipSchemaErrorConservativeIdentityLookupScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "selector_model_group",
			err:  errors.New("prepare schema: schema validation failed: resolve selector xpath './/kid' branch 1: element 'kid' not found in model group"),
		},
		{
			name: "selector_content_model",
			err:  errors.New("prepare schema: schema validation failed: resolve selector xpath './/kid' branch 1: element 'kid' not found in content model"),
		},
		{
			name: "field_model_group",
			err:  errors.New("prepare schema: schema validation failed: resolve field xpath 'myNS:col' branch 1: element '{myNS.tempuri.org}col' not found in model group"),
		},
		{
			name: "field_content_model",
			err:  errors.New("prepare schema: schema validation failed: resolve field xpath 'myNS:col' branch 1: element '{myNS.tempuri.org}col' not found in content model"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			skip, reason := shouldSkipSchemaError(tc.err)
			if !skip {
				t.Fatalf("shouldSkipSchemaError(%q) = false, want true", tc.err)
			}
			if reason == "" {
				t.Fatalf("shouldSkipSchemaError(%q) reason empty", tc.err)
			}
		})
	}
}
