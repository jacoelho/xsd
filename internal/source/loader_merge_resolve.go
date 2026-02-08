package source

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *SchemaLoader) mergeSchema(target, source *parser.Schema, kind loadmerge.Kind, remap loadmerge.NamespaceRemapMode, insertAt int) error {
	if l == nil {
		return fmt.Errorf("no loader configured")
	}
	merger := l.merger
	if merger == nil {
		merger = loadmerge.DefaultMerger{}
	}
	return merger.Merge(target, source, kind, remap, insertAt)
}

func (l *SchemaLoader) resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	if l == nil || l.resolver == nil {
		return nil, "", fmt.Errorf("no resolver configured")
	}
	return l.resolver.Resolve(req)
}
