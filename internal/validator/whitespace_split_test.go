package validator

import (
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestWhitespaceSplitConsistency(t *testing.T) {
	input := " \ta\r\nb\nc\t "

	var validatorTokens []string
	splitWhitespaceSeq(input, func(token string) bool {
		validatorTokens = append(validatorTokens, token)
		return true
	})

	var typesTokens []string
	for token := range types.FieldsXMLWhitespaceSeq(input) {
		typesTokens = append(typesTokens, token)
	}

	if !reflect.DeepEqual(validatorTokens, typesTokens) {
		t.Fatalf("whitespace split mismatch: %v vs %v", validatorTokens, typesTokens)
	}
}
