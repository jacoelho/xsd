package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXMLTextDecoderFileOwnership(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "pkg", "xmltext")

	required := []string{
		"decoder.go",
		"decoder_options.go",
		"decoder_encoding.go",
		"decoder_state.go",
		"decoder_buffer.go",
		"scanner_chardata.go",
		"scanner_markup.go",
		"scanner_name.go",
		"decoder_internal.go",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing xmltext file %s: %v", name, err)
		}
	}

	assertFileNotContains(t, filepath.Join(dir, "decoder.go"), "func resolveOptions(")
	assertFileNotContains(t, filepath.Join(dir, "decoder.go"), "func detectEncoding(")
	assertFileNotContains(t, filepath.Join(dir, "decoder.go"), "func wrapCharsetReaderFromBufio(")

	assertFileNotContains(t, filepath.Join(dir, "decoder_internal.go"), "func (d *Decoder) nextTokenInto(")
	assertFileNotContains(t, filepath.Join(dir, "decoder_internal.go"), "func (d *Decoder) ensureIndex(")
	assertFileNotContains(t, filepath.Join(dir, "decoder_internal.go"), "func (d *Decoder) resolveText(")
	assertFileNotContains(t, filepath.Join(dir, "decoder_internal.go"), "func (d *Decoder) scanStartTagInto(")
	assertFileNotContains(t, filepath.Join(dir, "decoder_internal.go"), "func (d *Decoder) scanQName(")
}

func TestXMLStreamReaderFileOwnership(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "pkg", "xmlstream")

	required := []string{
		"reader.go",
		"reader_core.go",
		"reader_text.go",
		"reader_start.go",
		"reader_end.go",
		"reader_namespace.go",
		"reader_value.go",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing xmlstream file %s: %v", name, err)
		}
	}

	assertFileNotContains(t, filepath.Join(dir, "reader.go"), "func (r *Reader) startEvent(")
	assertFileNotContains(t, filepath.Join(dir, "reader.go"), "func (r *Reader) endEvent(")
	assertFileNotContains(t, filepath.Join(dir, "reader.go"), "func (r *Reader) LookupNamespace(")
	assertFileNotContains(t, filepath.Join(dir, "reader.go"), "func (r *Reader) textBytes(")
}

func assertFileNotContains(t *testing.T, path, needle string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(data), needle) {
		t.Fatalf("%s unexpectedly contains %q", path, needle)
	}
}
