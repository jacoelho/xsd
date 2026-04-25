package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
	"github.com/jacoelho/xsd/internal/xsderrors"
)

// NewValidator creates a validator with explicit instance-validation config.
func (s *Schema) NewValidator(config ValidateConfig) (*Validator, error) {
	if s == nil || s.rt == nil {
		return nil, schemaNotLoadedError()
	}
	req, err := newValidateRequest(config)
	if err != nil {
		return nil, fmt.Errorf("validate schema: %w", err)
	}
	return req.newValidator(s.rt), nil
}

// Validate validates a document against the schema using default validate options.
func (s *Schema) Validate(r io.Reader) error {
	v, err := s.defaultValidatorForSchema()
	if err != nil {
		return err
	}
	return v.Validate(r)
}

// ValidateFSFile validates an XML file from the provided filesystem using default validate options.
func (s *Schema) ValidateFSFile(fsys fs.FS, path string) error {
	v, err := s.defaultValidatorForSchema()
	if err != nil {
		return err
	}
	return v.ValidateFSFile(fsys, path)
}

// ValidateFile validates an XML file against the schema using default validate options.
func (s *Schema) ValidateFile(path string) error {
	v, err := s.defaultValidatorForSchema()
	if err != nil {
		return err
	}
	return v.ValidateFile(path)
}

// Validate validates a document against the validator's compiled schema.
func (v *Validator) Validate(r io.Reader) error {
	return v.validateReader(r, "")
}

// ValidateFSFile validates an XML file from the provided filesystem.
func (v *Validator) ValidateFSFile(fsys fs.FS, path string) (err error) {
	return v.validateFile(path, func(filePath string) (io.ReadCloser, error) {
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

// ValidateFile validates an XML file against the validator's compiled schema.
func (v *Validator) ValidateFile(path string) (err error) {
	return v.validateFile(path, func(filePath string) (io.ReadCloser, error) {
		return os.Open(filePath)
	})
}

func (s *Schema) defaultValidatorForSchema() (*Validator, error) {
	if s == nil || s.rt == nil || s.defaultValidator == nil {
		return nil, schemaNotLoadedError()
	}
	return s.defaultValidator(), nil
}

func newValidator(rt *runtime.Schema, opts resolvedValidateOptions) *Validator {
	return &Validator{engine: validator.NewEngine(rt, opts.instanceParseOptions...)}
}

func (v *Validator) validateFile(path string, openFile func(string) (io.ReadCloser, error)) (err error) {
	if v == nil || v.engine == nil {
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

	return v.validateReader(f, path)
}

func (v *Validator) validateReader(r io.Reader, document string) error {
	if v == nil || v.engine == nil {
		return schemaNotLoadedError()
	}
	return v.engine.ValidateWithDocument(r, document)
}

func schemaNotLoadedError() error {
	return xsderrors.NewKind(xsderrors.KindCaller, xsderrors.ErrSchemaNotLoaded, "schema not loaded")
}
