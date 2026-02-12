package validator

import (
	"io"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// SessionIO owns reader state and parsing options for one validator session.
type SessionIO struct {
	reader        *xmlstream.Reader
	readerFactory func(io.Reader, ...xmlstream.Option) (*xmlstream.Reader, error)
	documentURI   string
	parseOptions  []xmlstream.Option
}
