package xsd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Compile loads, prepares, and builds one schema root with explicit source/build options.
func Compile(fsys fs.FS, location string, sourceOpts SourceOptions, buildOpts BuildOptions) (*Schema, error) {
	entry, err := newSourceEntry(fsys, location)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}

	prepared, err := preparePreparedSchema([]sourceEntry{entry}, sourceOpts)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	schema, err := prepared.Build(buildOpts)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	return schema, nil
}

// CompileFile loads, prepares, and builds one schema file with explicit source/build options.
func CompileFile(path string, sourceOpts SourceOptions, buildOpts BuildOptions) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return Compile(os.DirFS(dir), base, sourceOpts, buildOpts)
}
