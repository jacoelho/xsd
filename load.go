package xsd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LoadWithOptions loads and compiles a schema with explicit configuration.
func LoadWithOptions(fsys fs.FS, location string, opts LoadOptions) (*Schema, error) {
	entry, err := newSchemaSetEntry(fsys, location)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	schema, err := compileEntries([]schemaSetEntry{entry}, opts, nil)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	return schema, nil
}

// LoadFileWithOptions loads and compiles a schema from a file path with explicit configuration.
func LoadFileWithOptions(path string, opts LoadOptions) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return LoadWithOptions(os.DirFS(dir), base, opts)
}

// LoadFile loads and compiles a schema from a file path.
func LoadFile(path string) (*Schema, error) {
	return LoadFileWithOptions(path, NewLoadOptions())
}
