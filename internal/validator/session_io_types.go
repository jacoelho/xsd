package validator

import (
	"io"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type SessionIO struct {
	reader        *xmlstream.Reader
	readerFactory func(io.Reader, ...xmlstream.Option) (*xmlstream.Reader, error)
	documentURI   string
	parseOptions  []xmlstream.Option
}
