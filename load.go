package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/internal/compiler"
)

// Source identifies one schema root.
type Source struct {
	FS   fs.FS
	Path string
}

// Compiler compiles schemas with immutable configuration.
type Compiler struct {
	config CompileConfig
}

// NewCompiler returns a compiler with the provided configuration.
func NewCompiler(config CompileConfig) Compiler {
	return Compiler{config: config}
}

// CompileFS loads, prepares, and builds one schema root.
func CompileFS(fsys fs.FS, location string, config CompileConfig) (*Schema, error) {
	return NewCompiler(config).CompileFS(fsys, location)
}

// CompileFile loads, prepares, and builds one schema file.
func CompileFile(path string, config CompileConfig) (*Schema, error) {
	return NewCompiler(config).CompileFile(path)
}

// CompileFS loads, prepares, and builds one schema root.
func (c Compiler) CompileFS(fsys fs.FS, location string) (*Schema, error) {
	root, err := newCompileRoot(fsys, location)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	req, err := newCompileRequest([]compiler.Root{root}, c.config)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	schema, err := req.compile()
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", location, err)
	}
	return schema, nil
}

// CompileFile loads, prepares, and builds one schema file.
// The explicit entry path is loaded as requested, and nested include/import loads
// are confined to the selected file's containing directory tree.
func (c Compiler) CompileFile(path string) (*Schema, error) {
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
	req, err := newCompileRequest([]compiler.Root{compileRoot}, c.config)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	schema, err := req.compile()
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", base, err)
	}
	return schema, nil
}

// CompileSources loads, prepares, and builds multiple schema roots.
func (c Compiler) CompileSources(sources []Source) (*Schema, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("compile schema sources: no sources")
	}
	roots := make([]compiler.Root, 0, len(sources))
	for _, source := range sources {
		root, err := newCompileRoot(source.FS, source.Path)
		if err != nil {
			return nil, fmt.Errorf("compile schema source %s: %w", source.Path, err)
		}
		roots = append(roots, root)
	}
	req, err := newCompileRequest(roots, c.config)
	if err != nil {
		return nil, fmt.Errorf("compile schema sources: %w", err)
	}
	schema, err := req.compile()
	if err != nil {
		return nil, fmt.Errorf("compile schema sources: %w", err)
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
