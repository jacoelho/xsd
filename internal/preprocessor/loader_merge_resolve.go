package preprocessor

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) mergeSchema(target, source *parser.Schema, kind loadmerge.Kind, remap loadmerge.NamespaceRemapMode, insertAt int) error {
	if l == nil {
		return fmt.Errorf("no loader configured")
	}
	merger := l.merger
	if merger == nil {
		merger = loadmerge.DefaultMerger{}
	}
	return merger.Merge(
		target,
		source,
		kind,
		remap,
		insertAt,
	)
}
