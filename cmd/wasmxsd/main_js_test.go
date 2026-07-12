//go:build js && wasm

package main

import (
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
