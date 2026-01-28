package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jacoelho/xsd/errors"
)

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	engine *Engine
}

// Load loads and compiles a schema from the given filesystem and location.
func Load(fsys fs.FS, location string) (*Schema, error) {
	engine, err := CompileFS(fsys, location)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}
	return &Schema{engine: engine}, nil
}

// LoadFile loads and compiles a schema from a file path.
func LoadFile(path string) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return Load(os.DirFS(dir), base)
}

// Validate validates a document against the schema.
func (s *Schema) Validate(r io.Reader) error {
	if s == nil || s.engine == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
	}
	if r == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
	}
	return s.engine.Validate(r)
}

// ValidateFile validates an XML file against the schema.
func (s *Schema) ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open xml file %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close xml file %s: %w", path, closeErr)
		}
	}()

	return s.Validate(f)
}
