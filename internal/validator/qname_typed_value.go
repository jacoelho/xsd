package validator

import "github.com/jacoelho/xsd/internal/types"

type qnameTypedValue struct {
	typ     types.Type
	lexical string
	value   types.QName
}

func (v qnameTypedValue) Type() types.Type { return v.typ }

func (v qnameTypedValue) Lexical() string { return v.lexical }

func (v qnameTypedValue) Native() any { return v.value }

func (v qnameTypedValue) String() string {
	if v.lexical != "" {
		return v.lexical
	}
	return v.value.String()
}
