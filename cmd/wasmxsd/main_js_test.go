//go:build js && wasm

package main

import (
	"encoding/json"
	"strings"
	"syscall/js"
	"testing"
)

func TestJSStringArgumentAcceptsPrimitiveString(t *testing.T) {
	value, errText := jsStringArgument(js.ValueOf("root"), "XML", 4)
	if errText != "" || value != "root" {
		t.Fatalf("jsStringArgument() = %q, %q; want root, empty error", value, errText)
	}
}

func TestJSStringArgumentAllowsBoundedUTF8ExpansionForExactAPIByteCheck(t *testing.T) {
	value, errText := jsStringArgument(js.ValueOf("€"), "XML", 3)
	if errText != "" || value != "€" {
		t.Fatalf("jsStringArgument() = %q, %q; want euro sign, empty error", value, errText)
	}
}

func TestJSStringArgumentRejectsOversizePrimitiveBeforeConversion(t *testing.T) {
	value, errText := jsStringArgument(js.ValueOf("root"), "XML", 3)
	if value != "" || errText != "XML exceeds 3 bytes limit" {
		t.Fatalf("jsStringArgument() = %q, %q; want empty value and byte-limit error", value, errText)
	}
}

func TestJSStringArgumentRejectsNonStringBeforeBoxing(t *testing.T) {
	value, errText := jsStringArgument(js.ValueOf(42), "XML", 3)
	if value != "" || errText != "XML must be a string" {
		t.Fatalf("jsStringArgument() = %q, %q; want empty value and type error", value, errText)
	}
}

func TestFormatXMLJSRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []js.Value
		wantErr string
	}{
		{name: "wrong arity", wantErr: "invalid number of arguments"},
		{name: "non-string XML", args: []js.Value{js.ValueOf(42)}, wantErr: "XML must be a string"},
		{
			name:    "oversize XML",
			args:    []js.Value{js.ValueOf(strings.Repeat("x", int(maxXMLBytes)+1))},
			wantErr: "XML exceeds " + byteLimit(maxXMLBytes) + " limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response formatResponse
			decodeJSResponse(t, formatXMLJS(js.Undefined(), tt.args), &response)
			if response.Error != tt.wantErr {
				t.Fatalf("formatXMLJS() error = %q, want %q", response.Error, tt.wantErr)
			}
		})
	}
}

func TestFormatXMLJSReturnsSerializedFormatResponse(t *testing.T) {
	var response formatResponse
	decodeJSResponse(t, formatXMLJS(js.Undefined(), []js.Value{js.ValueOf(`<root/>`)}), &response)
	if response.Error != "" || response.XML != `<root></root>` {
		t.Fatalf("formatXMLJS() = %+v, want formatted XML response", response)
	}
}

func TestValidateXMLJSRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []js.Value
		wantErr string
	}{
		{name: "wrong arity", wantErr: "invalid number of arguments"},
		{
			name:    "non-string XML",
			args:    []js.Value{js.ValueOf(42), js.ValueOf(testSchema)},
			wantErr: "XML must be a string",
		},
		{
			name:    "non-string XSD",
			args:    []js.Value{js.ValueOf(`<root/>`), js.ValueOf(42)},
			wantErr: "XSD must be a string",
		},
		{
			name:    "oversize XML",
			args:    []js.Value{js.ValueOf(strings.Repeat("x", int(maxXMLBytes)+1)), js.ValueOf(testSchema)},
			wantErr: "XML exceeds " + byteLimit(maxXMLBytes) + " limit",
		},
		{
			name:    "oversize XSD",
			args:    []js.Value{js.ValueOf(`<root/>`), js.ValueOf(strings.Repeat("x", int(maxXSDBytes)+1))},
			wantErr: "XSD exceeds " + byteLimit(maxXSDBytes) + " limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response validateResponse
			decodeJSResponse(t, validateXMLJS(js.Undefined(), tt.args), &response)
			if response.Error != tt.wantErr {
				t.Fatalf("validateXMLJS() error = %q, want %q", response.Error, tt.wantErr)
			}
		})
	}
}

func TestValidateXMLJSReturnsSerializedValidationResponse(t *testing.T) {
	args := []js.Value{js.ValueOf(`<root><v>1</v></root>`), js.ValueOf(testSchema)}
	var response validateResponse
	decodeJSResponse(t, validateXMLJS(js.Undefined(), args), &response)
	if !response.Valid || response.Error != "" || len(response.Errors) != 0 {
		t.Fatalf("validateXMLJS() = %+v, want valid response", response)
	}
}

func decodeJSResponse(t *testing.T, value any, response any) {
	t.Helper()
	data, ok := value.(string)
	if !ok {
		t.Fatalf("JS callback response type = %T, want string", value)
	}
	if err := json.Unmarshal([]byte(data), response); err != nil {
		t.Fatalf("decode JS callback response %q: %v", data, err)
	}
}
