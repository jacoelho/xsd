package source

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func parseSchemaDocument(doc io.ReadCloser, systemID string, opts ...xmlstream.Option) (result *parser.ParseResult, err error) {
	if doc == nil {
		return nil, fmt.Errorf("nil schema reader")
	}
	defer func() {
		if closeErr := closeSchemaDoc(doc, systemID); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	result, err = parser.ParseWithImportsOptions(doc, opts...)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", systemID, err)
	}

	return result, nil
}

func closeSchemaDoc(doc io.Closer, systemID string) error {
	if err := doc.Close(); err != nil {
		return fmt.Errorf("close %s: %w", systemID, err)
	}
	return nil
}
