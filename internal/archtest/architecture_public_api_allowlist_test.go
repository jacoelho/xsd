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
		"const ErrCaller":                               {},
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
		"const ErrIO":                                   {},
		"const ErrIdentityAbsent":                       {},
		"const ErrIdentityDuplicate":                    {},
		"const ErrIdentityKeyRefFailed":                 {},
		"const ErrInternal":                             {},
		"const ErrMultipleIDAttr":                       {},
		"const ErrNilElementNotEmpty":                   {},
		"const ErrNoRoot":                               {},
		"const ErrRequiredAttributeMissing":             {},
		"const ErrRequiredElementMissing":               {},
		"const ErrSchemaNotLoaded":                      {},
		"const ErrSchemaParse":                          {},
		"const ErrSchemaSemantic":                       {},
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
		"const ErrValidationInternal":                   {},
		"const ErrWildcardNotDeclared":                  {},
		"const ErrXMLParse":                             {},
		"const ErrXsiTypeInvalid":                       {},
		"const KindCaller":                              {},
		"const KindInternal":                            {},
		"const KindIO":                                  {},
		"const KindSchema":                              {},
		"const KindValidation":                          {},
		"field Error.Actual":                            {},
		"field Error.Code":                              {},
		"field Error.Err":                               {},
		"field Error.Expected":                          {},
		"field Error.Kind":                              {},
		"field Error.Message":                           {},
		"field Validation.Actual":                       {},
		"field Validation.Code":                         {},
		"field Validation.Column":                       {},
		"field Validation.Document":                     {},
		"field Validation.Expected":                     {},
		"field Validation.Line":                         {},
		"field Validation.Message":                      {},
		"field Validation.Path":                         {},
		"func AsValidations":                            {},
		"func CompileFS":                                {},
		"func CompileFile":                              {},
		"func KindOf":                                   {},
		"func NewCompiler":                              {},
		"method Compiler.CompileFS":                     {},
		"method Compiler.CompileFile":                   {},
		"method Compiler.CompileSources":                {},
		"method Error.Error":                            {},
		"method Error.Is":                               {},
		"method Error.Unwrap":                           {},
		"method Name.IsZero":                            {},
		"method Name.String":                            {},
		"method Schema.NewValidator":                    {},
		"method Schema.Validate":                        {},
		"method Schema.ValidateFSFile":                  {},
		"method Schema.ValidateFile":                    {},
		"method Validation.Error":                       {},
		"method ValidationList.Error":                   {},
		"method ValidationList.Sort":                    {},
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

func TestPublicAPIHasNoInternalAliases(t *testing.T) {
	got := collectRootInternalAliases(t)
	for alias := range got {
		t.Errorf("unexpected public internal alias: %s", alias)
	}
}
