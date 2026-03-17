package preprocessor

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) mergeSchema(target, source *parser.Schema, kind MergeKind, remap NamespaceRemapMode, insertAt int) error {
	if l == nil {
		return fmt.Errorf("no loader configured")
	}
	return MergeSchema(target, source, kind, remap, insertAt)
}
