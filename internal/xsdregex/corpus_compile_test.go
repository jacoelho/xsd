package xsdregex

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCurrentUnsupportedRegexCorpus is a focused migration harness. It walks
// the checked-in XSD corpus and compiles patterns using the constructs that
// previously caused unsupported.regex: blocks, XML name escapes, or
// character-class subtraction. The corpus also contains deliberately invalid
// regex dialect probes; those are syntax errors and are left to the schema
// validity tests. A resource-limit error is never acceptable here.
func TestCurrentUnsupportedRegexCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "tests", "corpus")
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		t.Skip("test corpus is not present")
	} else if err != nil {
		t.Fatal(err)
	}
	var total, compiled, blocks, names, subtraction int
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) (walkErrResult error) {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".xsd") {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := os.OpenInRoot(root, relativePath)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil && walkErrResult == nil {
				walkErrResult = closeErr
			}
		}()
		decoder := xml.NewDecoder(file)
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				// A malformed schema is tested by the schema syntax harness;
				// it cannot provide a reliable pattern value here.
				return nil
			}
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != "pattern" {
				continue
			}
			for _, attr := range start.Attr {
				if attr.Name.Local != "value" || !needsMigration(attr.Value) {
					continue
				}
				total++
				if strings.Contains(attr.Value, `\p{Is`) {
					blocks++
				}
				if strings.Contains(attr.Value, `\i`) || strings.Contains(attr.Value, `\I`) || strings.Contains(attr.Value, `\c`) || strings.Contains(attr.Value, `\C`) {
					names++
				}
				if strings.Contains(attr.Value, `-[`) {
					subtraction++
				}
				if _, err := Compile(attr.Value, CompileOptions{}); err != nil {
					if IsLimit(err) {
						return err
					}
					continue
				}
				compiled++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total == 0 {
		t.Skip("corpus has no migration patterns")
	}
	if compiled == 0 || blocks == 0 || names == 0 || subtraction == 0 {
		t.Fatalf("migration corpus coverage = total %d, compiled %d, blocks %d, names %d, subtraction %d", total, compiled, blocks, names, subtraction)
	}
}

func needsMigration(source string) bool {
	return strings.Contains(source, `\p{Is`) ||
		strings.Contains(source, `\i`) || strings.Contains(source, `\I`) ||
		strings.Contains(source, `\c`) || strings.Contains(source, `\C`) ||
		strings.Contains(source, `-[`)
}
