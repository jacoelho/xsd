package model

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
)

func validateWhitespaceSeparatedList(typeName string, itemValidator TypeValidator, lexical string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(lexical) {
		count++
		if err := itemValidator(part); err != nil {
			return fmt.Errorf("invalid %s: %w", typeName, err)
		}
	}
	if count == 0 {
		return fmt.Errorf("invalid %s: empty value", typeName)
	}
	return nil
}

func validateIDREFS(lexical string) error {
	return validateWhitespaceSeparatedList("IDREFS", value.ValidateXSDNCName, lexical)
}

func validateENTITIES(lexical string) error {
	return validateWhitespaceSeparatedList("ENTITIES", value.ValidateXSDNCName, lexical)
}

func validateNMTOKENS(lexical string) error {
	return validateWhitespaceSeparatedList("NMTOKENS", value.ValidateXSDNMTOKEN, lexical)
}
