package xsd

import "testing"

func TestBase64BinaryLengthMatchesDecode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
		size  uint32
	}{
		{name: "empty", value: "", valid: true, size: 0},
		{name: "no padding", value: "AQID", valid: true, size: 3},
		{name: "xml whitespace", value: "A Q\r\nI\tD", valid: true, size: 3},
		{name: "one padding", value: "AQI=", valid: true, size: 2},
		{name: "two padding", value: "AQ==", valid: true, size: 1},
		{name: "invalid length", value: "AQI", valid: false},
		{name: "bad character", value: "AQ$D", valid: false},
		{name: "data after padding", value: "AQ=I", valid: false},
		{name: "too much padding", value: "A===", valid: false},
		{name: "non-zero one-pad bits", value: "AQJ=", valid: false},
		{name: "non-zero two-pad bits", value: "AB==", valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			length, lengthErr := base64BinaryLength(tt.value)
			decoded, decodeErr := decodeXSDBase64(tt.value)
			if tt.valid {
				if lengthErr != nil {
					t.Fatalf("base64BinaryLength() error = %v", lengthErr)
				}
				if decodeErr != nil {
					t.Fatalf("decodeXSDBase64() error = %v", decodeErr)
				}
				if length != tt.size {
					t.Fatalf("base64BinaryLength() = %d, want %d", length, tt.size)
				}
				if uint32(len(decoded)) != tt.size {
					t.Fatalf("decoded length = %d, want %d", len(decoded), tt.size)
				}
				return
			}
			if lengthErr == nil {
				t.Fatal("base64BinaryLength() error = nil")
			}
			if decodeErr == nil {
				t.Fatal("decodeXSDBase64() error = nil")
			}
		})
	}
}
