package types

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestTemporalParseValidateConsistency(t *testing.T) {
	type caseDef struct {
		name    string
		value   string
		wantErr bool
	}

	tests := []struct {
		name     string
		parse    func(string) error
		validate func(string) error
		cases    []caseDef
	}{
		{
			name: "dateTime",
			parse: func(value string) error {
				_, err := ParseDateTime(value)
				return err
			},
			validate: validateDateTime,
			cases: []caseDef{
				{name: "utc", value: "2001-01-01T00:00:00Z", wantErr: false},
				{name: "max-offset", value: "2001-01-01T00:00:00+14:00", wantErr: false},
				{name: "offset-too-large", value: "2001-01-01T00:00:00+14:01", wantErr: true},
				{name: "day-offset", value: "2001-01-01T24:00:00", wantErr: false},
				{name: "day-offset-fraction", value: "2001-01-01T24:00:00.1", wantErr: true},
				{name: "year-zero", value: "0000-01-01T00:00:00", wantErr: true},
				{name: "fraction-too-long", value: "2001-01-01T00:00:00.1234567890", wantErr: true},
			},
		},
		{
			name: "time",
			parse: func(lexical string) error {
				_, err := value.ParseTime([]byte(lexical))
				return err
			},
			validate: validateTime,
			cases: []caseDef{
				{name: "max-offset", value: "13:20:00+14:00", wantErr: false},
				{name: "offset-too-large", value: "13:20:00+14:01", wantErr: true},
				{name: "day-offset", value: "24:00:00", wantErr: false},
				{name: "day-offset-fraction", value: "24:00:00.1", wantErr: true},
				{name: "fraction-too-long", value: "13:20:00.1234567890", wantErr: true},
			},
		},
		{
			name: "date",
			parse: func(lexical string) error {
				_, err := value.ParseDate([]byte(lexical))
				return err
			},
			validate: validateDate,
			cases: []caseDef{
				{name: "utc", value: "2001-01-01Z", wantErr: false},
				{name: "max-offset", value: "2001-01-01+14:00", wantErr: false},
				{name: "offset-too-large", value: "2001-01-01+14:01", wantErr: true},
				{name: "year-zero", value: "0000-01-01", wantErr: true},
			},
		},
		{
			name: "gYear",
			parse: func(lexical string) error {
				_, err := value.ParseGYear([]byte(lexical))
				return err
			},
			validate: validateGYear,
			cases: []caseDef{
				{name: "utc", value: "2001Z", wantErr: false},
				{name: "max-offset", value: "2001+14:00", wantErr: false},
				{name: "offset-too-large", value: "2001+14:01", wantErr: true},
				{name: "year-zero", value: "0000", wantErr: true},
			},
		},
		{
			name: "gYearMonth",
			parse: func(lexical string) error {
				_, err := value.ParseGYearMonth([]byte(lexical))
				return err
			},
			validate: validateGYearMonth,
			cases: []caseDef{
				{name: "utc", value: "2001-10Z", wantErr: false},
				{name: "max-offset", value: "2001-10+14:00", wantErr: false},
				{name: "offset-too-large", value: "2001-10+14:01", wantErr: true},
			},
		},
		{
			name: "gMonth",
			parse: func(lexical string) error {
				_, err := value.ParseGMonth([]byte(lexical))
				return err
			},
			validate: validateGMonth,
			cases: []caseDef{
				{name: "utc", value: "--03Z", wantErr: false},
				{name: "max-offset", value: "--03+14:00", wantErr: false},
				{name: "offset-too-large", value: "--03+14:01", wantErr: true},
			},
		},
		{
			name: "gMonthDay",
			parse: func(lexical string) error {
				_, err := value.ParseGMonthDay([]byte(lexical))
				return err
			},
			validate: validateGMonthDay,
			cases: []caseDef{
				{name: "utc", value: "--03-15Z", wantErr: false},
				{name: "max-offset", value: "--03-15+14:00", wantErr: false},
				{name: "offset-too-large", value: "--03-15+14:01", wantErr: true},
			},
		},
		{
			name: "gDay",
			parse: func(lexical string) error {
				_, err := value.ParseGDay([]byte(lexical))
				return err
			},
			validate: validateGDay,
			cases: []caseDef{
				{name: "utc", value: "---15Z", wantErr: false},
				{name: "max-offset", value: "---15+14:00", wantErr: false},
				{name: "offset-too-large", value: "---15+14:01", wantErr: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.cases {
				t.Run(tc.name, func(t *testing.T) {
					parseErr := tt.parse(tc.value)
					validateErr := tt.validate(tc.value)
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
