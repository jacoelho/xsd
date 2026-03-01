package xsd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LoadWithOptions loads and compiles a schema with explicit configuration.
func LoadWithOptions(fsys fs.FS, location string, opts LoadOptions) (*Schema, error) {
	set := NewSchemaSet().WithLoadOptions(opts)
	if err := set.AddFS(fsys, location); err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	schema, err := set.Compile()
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	return schema, nil
}

// LoadFile loads and compiles a schema from a file path.
func LoadFile(path string) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return LoadWithOptions(os.DirFS(dir), base, NewLoadOptions())
}
