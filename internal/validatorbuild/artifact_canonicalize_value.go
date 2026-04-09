package validatorbuild

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (c *artifactCompiler) comparableValue(lexical string, typ model.Type) (model.ComparableValue, error) {
	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}
	return comparableForPrimitiveName(primName, lexical, c.res.isIntegerDerived(typ))
}

func (c *artifactCompiler) normalizeLexical(lexical string, typ model.Type) string {
	ws := c.res.whitespaceMode(typ)
	if ws == runtime.WSPreserve || lexical == "" {
		return lexical
	}
	normalized := value.NormalizeWhitespace(valueWhitespaceMode(ws), []byte(lexical), nil)
	return string(normalized)
}
