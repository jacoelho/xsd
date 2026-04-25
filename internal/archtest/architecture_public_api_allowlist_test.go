package archtest_test

import (
	"testing"
)

func TestPublicAPIAllowlist(t *testing.T) {
	got := collectRootExports(t)
	want := map[string]struct{}{
		"type BuildConfig":                              {},
		"type CompileConfig":                            {},
		"type Compiler":                                 {},
		"type Error":                                    {},
		"type ErrorCode":                                {},
		"type ErrorKind":                                {},
		"type LocalName":                                {},
		"type Name":                                     {},
		"type NamespaceURI":                             {},
		"type Schema":                                   {},
		"type Source":                                   {},
		"type SourceConfig":                             {},
		"type ValidateConfig":                           {},
		"type Validation":                               {},
		"type ValidationList":                           {},
		"type Validator":                                {},
		"type XMLConfig":                                {},
		"const ErrAttributeFixedValue":                  {},
		"const ErrAttributeNotDeclared":                 {},
		"const ErrAttributeProhibited":                  {},
		"const ErrContentModelInvalid":                  {},
		"const ErrDatatypeInvalid":                      {},
		"const ErrDuplicateID":                          {},
		"const ErrElementAbstract":                      {},
		"const ErrElementFixedValue":                    {},
		"const ErrElementNotDeclared":                   {},
		"const ErrElementNotNillable":                   {},
		"const ErrElementTypeAbstract":                  {},
		"const ErrFacetViolation":                       {},
		"const ErrIDRefNotFound":                        {},
		"const ErrIdentityAbsent":                       {},
		"const ErrIdentityDuplicate":                    {},
		"const ErrIdentityKeyRefFailed":                 {},
		"const ErrMultipleIDAttr":                       {},
		"const ErrNilElementNotEmpty":                   {},
		"const ErrNoRoot":                               {},
		"const ErrRequiredAttributeMissing":             {},
		"const ErrRequiredElementMissing":               {},
		"const ErrSchemaNotLoaded":                      {},
		"const ErrTextInElementOnly":                    {},
		"const ErrUnexpectedElement":                    {},
		"const ErrValidateElementAbstract":              {},
		"const ErrValidateNilledHasFixed":               {},
		"const ErrValidateNilledNotEmpty":               {},
		"const ErrValidateRootNotDeclared":              {},
		"const ErrValidateSimpleTypeAttrNotAllowed":     {},
		"const ErrValidateValueFacet":                   {},
		"const ErrValidateValueInvalid":                 {},
		"const ErrValidateWildcardAttrStrictUnresolved": {},
		"const ErrValidateWildcardElemStrictUnresolved": {},
		"const ErrValidateXsiNilNotNillable":            {},
		"const ErrValidateXsiTypeDerivationBlocked":     {},
		"const ErrValidateXsiTypeUnresolved":            {},
		"const ErrWildcardNotDeclared":                  {},
		"const ErrXMLParse":                             {},
		"const ErrXsiTypeInvalid":                       {},
		"const KindCaller":                              {},
		"const KindInternal":                            {},
		"const KindIO":                                  {},
		"const KindSchema":                              {},
		"const KindValidation":                          {},
		"func AsValidations":                            {},
		"func CompileFS":                                {},
		"func CompileFile":                              {},
		"func KindOf":                                   {},
		"func NewCompiler":                              {},
		"method Compiler.CompileFS":                     {},
		"method Compiler.CompileFile":                   {},
		"method Compiler.CompileSources":                {},
		"method Name.IsZero":                            {},
		"method Name.String":                            {},
		"method Schema.NewValidator":                    {},
		"method Schema.Validate":                        {},
		"method Schema.ValidateFSFile":                  {},
		"method Schema.ValidateFile":                    {},
		"method Validator.Validate":                     {},
		"method Validator.ValidateFSFile":               {},
		"method Validator.ValidateFile":                 {},
	}

	for item := range want {
		if _, ok := got[item]; !ok {
			t.Errorf("missing public export: %s", item)
		}
	}
	for item := range got {
		if _, ok := want[item]; !ok {
			t.Errorf("unexpected public export: %s", item)
		}
	}
}
