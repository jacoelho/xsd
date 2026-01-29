package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// lookupTypeInSchema finds a type by QName in schema or builtins.
// Returns an error if type is not found.
func lookupTypeInSchema(schema *parser.Schema, qname types.QName) (types.Type, error) {
	if builtinType := types.GetBuiltinNS(qname.Namespace, qname.Local); builtinType != nil {
		if qname.Local == "anyType" {
			return types.NewAnyTypeComplexType(), nil
		}
		return builtinType, nil
	}

	if typeDef, ok := schema.TypeDefs[qname]; ok {
		return typeDef, nil
	}

	// type not found
	return nil, fmt.Errorf("type %s not found", qname)
}
