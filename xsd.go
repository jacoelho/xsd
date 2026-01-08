package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/validator"
)

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	compiled          *grammar.CompiledSchema
	validatorOnce     sync.Once
	validatorInstance *validator.Validator
}

// Load loads and compiles a schema from the given filesystem and location.
func Load(fsys fs.FS, location string) (*Schema, error) {
	l := loader.NewLoader(loader.Config{
		FS: fsys,
	})

	compiled, err := l.LoadCompiled(location)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", location, err)
	}

	return &Schema{compiled: compiled}, nil
}

// LoadFile loads and compiles a schema from a file path.
func LoadFile(path string) (*Schema, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	return Load(os.DirFS(dir), base)
}

// Validate validates a document against the schema.
func (s *Schema) Validate(r io.Reader) error {
	if s == nil || s.compiled == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
	}
	if r == nil {
		return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
	}

	v := s.getValidator()
	violations, err := v.ValidateStream(r)
	if err != nil {
		if list, ok := errors.AsValidations(err); ok {
			return errors.ValidationList(list)
		}
		return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, err.Error(), "")}
	}
	if len(violations) == 0 {
		return nil
	}
	return errors.ValidationList(violations)
}

func (s *Schema) getValidator() *validator.Validator {
	if s == nil {
		return nil
	}
	s.validatorOnce.Do(func() {
		s.validatorInstance = validator.New(s.compiled)
	})
	return s.validatorInstance
}

// ValidateFile validates an XML file against the schema.
func (s *Schema) ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open xml file %s: %w", path, err)
	}
	defer f.Close()

	return s.Validate(f)
}
