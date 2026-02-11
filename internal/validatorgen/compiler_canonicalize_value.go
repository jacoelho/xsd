package validatorgen

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuesemantics"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func (c *compiler) comparableValue(lexical string, typ model.Type) (model.ComparableValue, error) {
	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}
	return valuesemantics.ComparableForPrimitiveName(primName, lexical, c.res.isIntegerDerived(typ))
}

func (c *compiler) normalizeLexical(lexical string, typ model.Type) string {
	ws := c.res.whitespaceMode(typ)
	if ws == runtime.WS_Preserve || lexical == "" {
		return lexical
	}
	normalized := value.NormalizeWhitespace(wsmode.ToValue(ws), []byte(lexical), nil)
	return string(normalized)
}
