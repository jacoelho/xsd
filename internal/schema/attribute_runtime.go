package schema

import "github.com/jacoelho/xsd/xsderrors"

func invalidAttributeError(err error) error {
	return compileRuleError(xsderrors.CodeSchemaInvalidAttribute, "", err)
}
