package internalcore

import "fmt"

type NormalizeOps struct {
	IsNilType         func(typ any) bool
	IsBuiltinType     func(typ any) bool
	TypeNameLocal     func(typ any) string
	PrimitiveType     func(typ any) any
	WhiteSpaceMode    func(typ any) int
	ApplyWhiteSpace   func(lexical string, whiteSpaceMode int) string
	TrimXMLWhitespace func(lexical string) string
}

var temporalTypeNames = map[string]struct{}{
	"dateTime":   {},
	"date":       {},
	"time":       {},
	"gYearMonth": {},
	"gYear":      {},
	"gMonthDay":  {},
	"gDay":       {},
	"gMonth":     {},
}

// NormalizeValue normalizes a lexical value using type and whitespace callbacks.
func NormalizeValue(lexical string, typ any, ops NormalizeOps) (string, error) {
	if typ == nil || ops.IsNilType(typ) {
		return lexical, fmt.Errorf("cannot normalize value for nil type")
	}

	normalized := ops.ApplyWhiteSpace(lexical, ops.WhiteSpaceMode(typ))
	if isTemporalType(typeNameForNormalization(typ, ops)) {
		return ops.TrimXMLWhitespace(normalized), nil
	}
	return normalized, nil
}

func typeNameForNormalization(typ any, ops NormalizeOps) string {
	if ops.IsBuiltinType(typ) {
		return ops.TypeNameLocal(typ)
	}
	primitive := ops.PrimitiveType(typ)
	if primitive == nil || ops.IsNilType(primitive) {
		return ""
	}
	return ops.TypeNameLocal(primitive)
}

func isTemporalType(typeName string) bool {
	_, ok := temporalTypeNames[typeName]
	return ok
}
