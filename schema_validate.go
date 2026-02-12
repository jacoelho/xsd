package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
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
		return fsys.Open(filePath)
	}, func(reader io.Reader, document string) error {
		return s.validateReader(reader, document)
	})
}

// ValidateFile validates an XML file against the schema.
func (s *Schema) ValidateFile(path string) (err error) {
	return s.validateFile(path, func(filePath string) (io.ReadCloser, error) {
		return os.Open(filePath)
	}, func(reader io.Reader, document string) error {
		return s.validateReader(reader, document)
	})
}

func (s *Schema) validateFile(path string, openFile func(string) (io.ReadCloser, error), validate func(io.Reader, string) error) (err error) {
	if s == nil || s.engine == nil {
		return schemaNotLoadedError()
	}
	if openFile == nil {
		return fmt.Errorf("open xml file %s: nil opener", path)
	}
	if validate == nil {
		return fmt.Errorf("validate xml file %s: nil validator", path)
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

	err = validate(f, path)
	return err
}

func (s *Schema) validateReader(r io.Reader, document string) error {
	var eng *engine
	if s != nil {
		eng = s.engine
	}
	return eng.validateDocument(r, document)
}
