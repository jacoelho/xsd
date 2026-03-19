package preprocessor

import (
	"errors"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

// LoadCallbacks supplies the root-owned graph lookups and state transitions
// needed to turn one resolved document into a loaded schema.
type LoadCallbacks struct {
	Loaded      func() (*parser.Schema, bool)
	Circular    func() (*parser.Schema, error)
	Close       func(io.Closer, string) error
	Parse       func(io.ReadCloser, string) (*parser.ParseResult, error)
	ApplyParsed func(*parser.ParseResult, string) (*parser.Schema, error)
}

// LoadResolved handles the generic cached/circular/parse/apply flow for one
// already-resolved schema document.
func LoadResolved(doc io.ReadCloser, systemID string, callbacks LoadCallbacks) (*parser.Schema, error) {
	if callbacks.Loaded != nil {
		if sch, ok := callbacks.Loaded(); ok {
			if err := callbacks.Close(doc, systemID); err != nil {
				return nil, err
			}
			return sch, nil
		}
	}

	if callbacks.Circular != nil {
		sch, err := callbacks.Circular()
		if err != nil || sch != nil {
			return sch, errors.Join(err, callbacks.Close(doc, systemID))
		}
	}

	result, err := callbacks.Parse(doc, systemID)
	if err != nil {
		return nil, err
	}
	return callbacks.ApplyParsed(result, systemID)
}
