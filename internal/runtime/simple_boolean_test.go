package runtime

import "testing"

func TestParseBooleanValueStringAndBytesMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lexical string
		want    bool
		wantErr string
	}{
		{lexical: "true", want: true},
		{lexical: "1", want: true},
		{lexical: "false"},
		{lexical: "0"},
		{lexical: "", wantErr: "invalid boolean"},
		{lexical: "TRUE", wantErr: "invalid boolean"},
		{lexical: " yes ", wantErr: "invalid boolean"},
	}
	for _, tt := range tests {
		t.Run(tt.lexical, func(t *testing.T) {
			t.Parallel()

			stringValue, stringErr := ParseBooleanValue(tt.lexical)
			bytesValue, bytesErr := ParseBooleanValue([]byte(tt.lexical))
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ParseBooleanValue string error = %v, bytes error = %v", stringErr, bytesErr)
			}
			if stringValue != bytesValue {
				t.Fatalf("ParseBooleanValue string value = %v, bytes value = %v", stringValue, bytesValue)
			}
			if tt.wantErr == "" {
				if stringErr != nil {
					t.Fatalf("ParseBooleanValue(%q) error = %v", tt.lexical, stringErr)
				}
				if stringValue != tt.want {
					t.Fatalf("ParseBooleanValue(%q) = %v, want %v", tt.lexical, stringValue, tt.want)
				}
				return
			}
			if stringErr == nil || stringErr.Error() != tt.wantErr {
				t.Fatalf("ParseBooleanValue(%q) error = %v, want %q", tt.lexical, stringErr, tt.wantErr)
			}
		})
	}
}
