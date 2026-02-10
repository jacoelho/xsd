package model

import "testing"

func TestParseValidateConsistency_Primitives(t *testing.T) {
	type caseDef struct {
		name    string
		value   string
		wantErr bool
	}

	tests := []struct {
		name     string
		typeName TypeName
		parse    func(string) error
		validate func(string) error
		cases    []caseDef
	}{
		{
			name:     "decimal",
			typeName: TypeNameDecimal,
			parse: func(value string) error {
				_, err := ParseDecimal(value)
				return err
			},
			validate: validateDecimal,
			cases: []caseDef{
				{name: "valid", value: "1.0", wantErr: false},
				{name: "leading-dot", value: ".5", wantErr: false},
				{name: "empty", value: "", wantErr: true},
				{name: "whitespace", value: " 10 ", wantErr: false},
			},
		},
		{
			name:     "unsignedLong",
			typeName: TypeNameUnsignedLong,
			parse: func(value string) error {
				_, err := ParseUnsignedLong(value)
				return err
			},
			validate: validateUnsignedLong,
			cases: []caseDef{
				{name: "max", value: "18446744073709551615", wantErr: false},
				{name: "overflow", value: "18446744073709551616", wantErr: true},
				{name: "plus", value: "+1", wantErr: false},
				{name: "minus-zero", value: "-0", wantErr: false},
				{name: "minus", value: "-1", wantErr: true},
				{name: "whitespace", value: " 42 ", wantErr: false},
			},
		},
		{
			name:     "unsignedInt",
			typeName: TypeNameUnsignedInt,
			parse: func(value string) error {
				_, err := ParseUnsignedInt(value)
				return err
			},
			validate: validateUnsignedInt,
			cases: []caseDef{
				{name: "max", value: "4294967295", wantErr: false},
				{name: "overflow", value: "4294967296", wantErr: true},
				{name: "plus", value: "+1", wantErr: false},
				{name: "minus", value: "-1", wantErr: true},
			},
		},
		{
			name:     "unsignedShort",
			typeName: TypeNameUnsignedShort,
			parse: func(value string) error {
				_, err := ParseUnsignedShort(value)
				return err
			},
			validate: validateUnsignedShort,
			cases: []caseDef{
				{name: "max", value: "65535", wantErr: false},
				{name: "overflow", value: "65536", wantErr: true},
				{name: "plus", value: "+1", wantErr: false},
				{name: "minus", value: "-1", wantErr: true},
			},
		},
		{
			name:     "unsignedByte",
			typeName: TypeNameUnsignedByte,
			parse: func(value string) error {
				_, err := ParseUnsignedByte(value)
				return err
			},
			validate: validateUnsignedByte,
			cases: []caseDef{
				{name: "max", value: "255", wantErr: false},
				{name: "overflow", value: "256", wantErr: true},
				{name: "plus", value: "+1", wantErr: false},
				{name: "minus", value: "-1", wantErr: true},
			},
		},
		{
			name:     "anyURI",
			typeName: TypeNameAnyURI,
			parse: func(value string) error {
				_, err := ParseAnyURI(value)
				return err
			},
			validate: validateAnyURI,
			cases: []caseDef{
				{name: "valid", value: "http://example.com", wantErr: false},
				{name: "collapse", value: "http://ex\tample.com", wantErr: false},
				{name: "bad-percent", value: "http://example.com/%G1", wantErr: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := mustBuiltinSimpleType(t, tt.typeName)
			for _, tc := range tt.cases {
				t.Run(tc.name, func(t *testing.T) {
					normalized, err := normalizeValue(tc.value, typ)
					if err != nil {
						t.Fatalf("normalizeValue(%q) error = %v", tc.value, err)
					}
					parseErr := tt.parse(normalized)
					validateErr := tt.validate(normalized)
					if (parseErr != nil) != tc.wantErr {
						t.Fatalf("parse error = %v, wantErr %v", parseErr, tc.wantErr)
					}
					if (validateErr != nil) != tc.wantErr {
						t.Fatalf("validate error = %v, wantErr %v", validateErr, tc.wantErr)
					}
					if (parseErr != nil) != (validateErr != nil) {
						t.Fatalf("parse/validate mismatch: parse=%v validate=%v", parseErr, validateErr)
					}
				})
			}
		})
	}
}
