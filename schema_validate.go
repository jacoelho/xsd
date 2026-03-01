package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/jacoelho/xsd/errors"
)

// Validate validates a document against the schema.
func (s *Schema) Validate(r io.Reader) error {
	return s.validateReader(r, "")
}

// ValidateFSFile validates an XML file from the provided filesystem.
func (s *Schema) ValidateFSFile(fsys fs.FS, path string) (err error) {
	return s.validateFile(path, func(filePath string) (io.ReadCloser, error) {
		if fsys == nil {
			return nil, fmt.Errorf("nil fs")
		}
		f, openErr := fsys.Open(filePath)
		if openErr != nil {
			return nil, openErr
		}
		return f, nil
	})
}

// ValidateFile validates an XML file against the schema.
func (s *Schema) ValidateFile(path string) (err error) {
	return s.validateFile(path, func(filePath string) (io.ReadCloser, error) {
		return os.Open(filePath)
	})
}

func (s *Schema) validateFile(path string, openFile func(string) (io.ReadCloser, error)) (err error) {
	if s == nil || s.engine == nil {
		return schemaNotLoadedError()
	}
	if openFile == nil {
		return fmt.Errorf("open xml file %s: nil opener", path)
	}

	f, err := openFile(path)
	if err != nil {
		return fmt.Errorf("open xml file %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close xml file %s: %w", path, closeErr)
		}
	}()

	err = s.validateReader(f, path)
	return err
}

func (s *Schema) validateReader(r io.Reader, document string) error {
	if s == nil || s.engine == nil {
		return schemaNotLoadedError()
	}
	return s.engine.ValidateWithDocument(r, document)
}

func schemaNotLoadedError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
}
