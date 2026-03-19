package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/internal/preprocessor/resolve"
)

// Compile loads, prepares, and builds one schema root with explicit source/build options.
func Compile(fsys fs.FS, location string, sourceOpts SourceOptions, buildOpts BuildOptions) (*Schema, error) {
	entry, err := newSourceEntry(fsys, location)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	return compileSourceEntry(entry, location, sourceOpts, buildOpts)
}

func compileSourceEntry(entry sourceEntry, displayLocation string, sourceOpts SourceOptions, buildOpts BuildOptions) (*Schema, error) {
	prepared, err := preparePreparedSchema([]sourceEntry{entry}, sourceOpts)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", displayLocation, err)
	}
	schema, err := prepared.Build(buildOpts)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", displayLocation, err)
	}
	return schema, nil
}

// CompileFile loads, prepares, and builds one schema file with explicit source/build options.
// The explicit entry path is loaded as requested, and nested include/import loads
// are confined to the selected file's containing directory tree.
func CompileFile(path string, sourceOpts SourceOptions, buildOpts BuildOptions) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	defer func() {
		_ = root.Close()
	}()

	entry, err := newSourceEntry(root.FS(), base)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	entry.resolver = &compileFileResolver{
		path:     path,
		systemID: base,
		nested:   resolve.NewFSResolver(root.FS()),
	}
	return compileSourceEntry(entry, base, sourceOpts, buildOpts)
}

type compileFileResolver struct {
	nested   resolve.Resolver
	path     string
	systemID string
}

func (r *compileFileResolver) Resolve(req resolve.Request) (io.ReadCloser, string, error) {
	if req.BaseSystemID == "" && req.SchemaLocation == r.systemID {
		f, err := os.Open(r.path)
		if err != nil {
			return nil, "", err
		}
		return f, r.systemID, nil
	}
	return r.nested.Resolve(req)
}
