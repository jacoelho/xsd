package internalcore

import "fmt"

type ValueParserFunc func(lexical string, typ any) (any, error)

// ParseValueForType parses lexical with the parser registered for typeName.
func ParseValueForType(lexical, typeName string, typ any, parsers map[string]ValueParserFunc) (any, error) {
	parser, ok := parsers[typeName]
	if !ok {
		return nil, fmt.Errorf("no parser for type %s", typeName)
	}
	return parser(lexical, typ)
}
