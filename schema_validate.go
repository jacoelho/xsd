package xsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
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
	engine, err := s.defaultEngineForSchema()
	if err != nil {
		return err
	}
	return validateReaderWithEngine(engine, r, "")
}

// ValidateFSFile validates an XML file from the provided filesystem using default validate options.
func (s *Schema) ValidateFSFile(fsys fs.FS, path string) error {
	engine, err := s.defaultEngineForSchema()
	if err != nil {
		return err
	}
	return validateFSFileWithEngine(engine, fsys, path)
}

// ValidateFile validates an XML file against the schema using default validate options.
func (s *Schema) ValidateFile(path string) error {
	engine, err := s.defaultEngineForSchema()
	if err != nil {
		return err
	}
	return validateFileWithEngine(engine, path, func(filePath string) (io.ReadCloser, error) {
		return os.Open(filePath)
	})
}

// Validate validates a document against the validator's compiled schema.
func (v *Validator) Validate(r io.Reader) error {
	return validateReaderWithEngine(v.engineForValidator(), r, "")
}

// ValidateFSFile validates an XML file from the provided filesystem.
func (v *Validator) ValidateFSFile(fsys fs.FS, path string) (err error) {
	return validateFSFileWithEngine(v.engineForValidator(), fsys, path)
}

// ValidateFile validates an XML file against the validator's compiled schema.
func (v *Validator) ValidateFile(path string) (err error) {
	return validateFileWithEngine(v.engineForValidator(), path, func(filePath string) (io.ReadCloser, error) {
		return os.Open(filePath)
	})
}

func (s *Schema) defaultEngineForSchema() (*validator.Engine, error) {
	if s == nil || s.rt == nil || s.defaultEngine == nil {
		return nil, schemaNotLoadedError()
	}
	return s.defaultEngine, nil
}

func newValidator(rt *runtime.Schema, opts resolvedValidateOptions) *Validator {
	return &Validator{engine: validator.NewEngine(rt, opts.instanceParseOptions...)}
}

func (v *Validator) engineForValidator() *validator.Engine {
	if v == nil {
		return nil
	}
	return v.engine
}

func validateFSFileWithEngine(engine *validator.Engine, fsys fs.FS, path string) error {
	if engine == nil {
		return schemaNotLoadedError()
	}
	if fsys == nil {
		return classifyCallerError(fmt.Errorf("nil fs"))
	}
	return validateFileWithEngine(engine, path, func(filePath string) (io.ReadCloser, error) {
		f, openErr := fsys.Open(filePath)
		if openErr != nil {
			return nil, openErr
		}
		return f, nil
	})
}

func validateFileWithEngine(engine *validator.Engine, path string, openFile func(string) (io.ReadCloser, error)) (err error) {
	if engine == nil {
		return schemaNotLoadedError()
	}
	if openFile == nil {
		return classifyInternalError(fmt.Errorf("open xml file %s: nil opener", path))
	}

	f, err := openFile(path)
	if err != nil {
		return classifyIOError(fmt.Errorf("open xml file %s: %w", path, err))
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = classifyIOError(fmt.Errorf("close xml file %s: %w", path, closeErr))
		}
	}()

	return validateReaderWithEngine(engine, f, path)
}

func validateReaderWithEngine(engine *validator.Engine, r io.Reader, document string) error {
	if engine == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return classifyCallerError(fmt.Errorf("nil reader"))
	}
	return classifyValidationBoundaryError(engine.ValidateWithDocument(r, document))
}

func schemaNotLoadedError() error {
	return Error{Kind: KindCaller, Code: ErrSchemaNotLoaded, Message: "schema not loaded"}
}
