package xsdregex

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestManifestRegexFixtures verifies the three architecture fixtures now
// compile as valid schemas and that their migrated regexes retain their
// intended positive and negative matches.
func TestManifestRegexFixtures(t *testing.T) {
	testsRoot := filepath.Join("..", "..", "tests")
	manifestFile, err := os.OpenInRoot(testsRoot, "manifest.json")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("test manifest is not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := manifestFile.Close(); closeErr != nil {
			t.Errorf("close test manifest: %v", closeErr)
		}
	}()
	var manifest struct {
		Cases []struct {
			Schema *struct {
				Expected  string `json:"expected"`
				ErrorCode string `json:"errorCode"`
			} `json:"schema"`
			ID string `json:"id"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(manifestFile).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		path     string
		positive []string
		negative []string
	}{
		"project/architecture-regex-class-subtraction-unsupported": {
			path:     "project/architecture-regex-class-subtraction-unsupported/schema.xsd",
			positive: []string{"rhythm", "bcdf"},
			negative: []string{"reader", "aeiou"},
		},
		"project/architecture-regex-i-escape-unsupported": {
			path:     "project/architecture-regex-i-escape-unsupported/schema.xsd",
			positive: []string{"Name_2", "éclair"},
			negative: []string{"2Name", ""},
		},
		"project/architecture-regex-unicode-block-unsupported": {
			path:     "project/architecture-regex-unicode-block-unsupported/schema.xsd",
			positive: []string{"abc123"},
			negative: []string{"é", "Ω"},
		},
	}
	found := 0
	for _, entry := range manifest.Cases {
		fixture, ok := want[entry.ID]
		if !ok {
			continue
		}
		found++
		if entry.Schema == nil || entry.Schema.Expected != "valid" || entry.Schema.ErrorCode != "" {
			t.Fatalf("manifest case %q changed outcome: schema=%+v", entry.ID, entry.Schema)
		}
		pattern := readPatternValue(t, fixture.path)
		compiled, err := Compile(pattern, CompileOptions{})
		if err != nil {
			t.Fatalf("Compile(%q from %s) error = %v", pattern, fixture.path, err)
		}
		for _, input := range fixture.positive {
			matched, err := compiled.MatchString(input)
			if err != nil || !matched {
				t.Errorf("%s: MatchString(%q) = %v, %v; want true", entry.ID, input, matched, err)
			}
		}
		for _, input := range fixture.negative {
			matched, err := compiled.MatchString(input)
			if err != nil || matched {
				t.Errorf("%s: MatchString(%q) = %v, %v; want false", entry.ID, input, matched, err)
			}
		}
	}
	if found != len(want) {
		t.Fatalf("manifest formerly unsupported regex cases = %d, want %d", found, len(want))
	}
}

func readPatternValue(t *testing.T, filename string) string {
	t.Helper()
	corpusRoot := filepath.Join("..", "..", "tests", "corpus")
	file, err := os.OpenInRoot(corpusRoot, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close corpus schema %q: %v", filename, closeErr)
		}
	}()
	decoder := xml.NewDecoder(file)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "pattern" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "value" {
				return attr.Value
			}
		}
	}
	t.Fatalf("%s has no pattern value", filename)
	return ""
}
