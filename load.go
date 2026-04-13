package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/internal/compiler"
)

// Compile loads, prepares, and builds one schema root with explicit source/build options.
func Compile(fsys fs.FS, location string, opts ...CompileOption) (*Schema, error) {
	root, err := newCompileRoot(fsys, location)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	req, err := newCompileRequest([]compiler.Root{root}, opts)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	schema, err := req.compile()
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	return schema, nil
}

// CompileFile loads, prepares, and builds one schema file with explicit source/build options.
// The explicit entry path is loaded as requested, and nested include/import loads
// are confined to the selected file's containing directory tree.
func CompileFile(path string, opts ...CompileOption) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	defer func() {
		_ = root.Close()
	}()

	compileRoot, err := newCompileRoot(root.FS(), base)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	compileRoot.Resolver = &compileFileResolver{
		path:     path,
		systemID: base,
		nested:   compiler.NewFSResolver(root.FS()),
	}
	req, err := newCompileRequest([]compiler.Root{compileRoot}, opts)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	schema, err := req.compile()
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	return schema, nil
}

type compileFileResolver struct {
	nested   compiler.SchemaResolver
	path     string
	systemID string
}

func (r *compileFileResolver) Resolve(req compiler.ResolveRequest) (io.ReadCloser, string, error) {
	if req.BaseSystemID == "" && req.SchemaLocation == r.systemID {
		f, err := os.Open(r.path)
		if err != nil {
			return nil, "", err
		}
		return f, r.systemID, nil
	}
	return r.nested.Resolve(req)
}
