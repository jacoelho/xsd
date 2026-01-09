package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// lookupType finds a type by QName in schema or builtins.
// Returns an error if type is not found.
func lookupType(schema *parser.Schema, qname types.QName) (types.Type, error) {
	if builtinType := types.GetBuiltinNS(qname.Namespace, qname.Local); builtinType != nil {
		if qname.Local == "anyType" {
			// anyType is a complex type
			ct := &types.ComplexType{
				QName: qname,
			}
			ct.SetContent(&types.EmptyContent{})
			ct.SetMixed(false)
			return ct, nil
		}
		return builtinType, nil
	}

	if typeDef, ok := schema.TypeDefs[qname]; ok {
		return typeDef, nil
	}

	// type not found
	return nil, fmt.Errorf("type %s not found", qname)
}
