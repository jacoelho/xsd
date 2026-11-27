package xsdregex

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/corpus"
)

// TestManifestRegexFixtures verifies the three architecture fixtures now
// compile as valid schemas and that their migrated regexes retain their
// intended positive and negative matches.
func TestManifestRegexFixtures(t *testing.T) {
	inventory, err := corpus.LoadInventory(os.DirFS(filepath.Join("..", "..", "tests")))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		path     string
		positive []string
		negative []string
	}{
		"project/architecture-regex-class-subtraction-unsupported": {
			path:     "project/datatypes/regex/architecture-regex-class-subtraction-unsupported/schema.xsd",
			positive: []string{"rhythm", "bcdf"},
			negative: []string{"reader", "aeiou"},
		},
		"project/architecture-regex-i-escape-unsupported": {
			path:     "project/datatypes/regex/architecture-regex-i-escape-unsupported/schema.xsd",
			positive: []string{"Name_2", "éclair"},
			negative: []string{"2Name", ""},
		},
		"project/architecture-regex-unicode-block-unsupported": {
			path:     "project/datatypes/regex/architecture-regex-unicode-block-unsupported/schema.xsd",
			positive: []string{"abc123"},
			negative: []string{"é", "Ω"},
		},
	}
	found := 0
	for _, entry := range inventory.Cases {
		fixture, ok := want[entry.ID]
		if !ok {
			continue
		}
		found++
		if entry.Schema == nil || entry.Schema.Expected != corpus.ExpectedValid || entry.Schema.ErrorCode != "" {
			t.Fatalf("manifest case %q changed outcome: schema=%+v", entry.ID, entry.Schema)
		}
		pattern := readPatternValue(t, fixture.path)
		for _, input := range fixture.positive {
			if !matchEvery(t, pattern, input) {
				t.Errorf("%s: %q does not match %q", entry.ID, pattern, input)
			}
		}
		for _, input := range fixture.negative {
			if matchEvery(t, pattern, input) {
				t.Errorf("%s: %q matches %q", entry.ID, pattern, input)
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
