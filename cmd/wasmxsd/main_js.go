//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
)

var jsFuncs []js.Func

type limitCatalog struct {
	MaxXMLBytes          int64 `json:"maxXMLBytes"`          //nolint:tagliatelle // Browser API uses conventional initialisms.
	MaxFormattedXMLBytes int64 `json:"maxFormattedXMLBytes"` //nolint:tagliatelle // Browser API uses conventional initialisms.
	MaxXSDBytes          int64 `json:"maxXSDBytes"`          //nolint:tagliatelle // Browser API uses conventional initialisms.
	MaxValidationErrors  int   `json:"maxValidationErrors"`
}

func main() {
	holdJSFunc("formatXML", formatXMLJS)
	holdJSFunc("validateXML", validateXMLJS)
	limitsJSON, err := json.Marshal(limitCatalog{
		MaxXMLBytes:          maxXMLBytes,
		MaxFormattedXMLBytes: maxFormattedXMLBytes,
		MaxXSDBytes:          maxXSDBytes,
		MaxValidationErrors:  maxValidationErrors,
	})
	if err != nil {
		panic("marshal browser limits: " + err.Error())
	}
	js.Global().Set("xsdLimits", string(limitsJSON))
	select {}
}

func holdJSFunc(name string, fn func(js.Value, []js.Value) any) {
	f := js.FuncOf(fn)
	jsFuncs = append(jsFuncs, f)
	js.Global().Set(name, f)
}

func formatXMLJS(this js.Value, args []js.Value) any {
	if len(args) != 1 {
		return marshalResponse(formatFailure("invalid number of arguments", 0, 0))
	}
	input, inputErr := jsStringArgument(args[0], "XML", maxXMLBytes)
	if inputErr != "" {
		return marshalResponse(formatFailure(inputErr, 0, 0))
	}
	return marshalResponse(formatXMLData(input))
}

func validateXMLJS(this js.Value, args []js.Value) any {
	if len(args) != 2 {
		return marshalResponse(validationFailure("invalid number of arguments"))
	}
	xmlText, xmlErr := jsStringArgument(args[0], "XML", maxXMLBytes)
	if xmlErr != "" {
		return marshalResponse(validationFailure(xmlErr))
	}
	xsdText, xsdErr := jsStringArgument(args[1], "XSD", maxXSDBytes)
	if xsdErr != "" {
		return marshalResponse(validationFailure(xsdErr))
	}
	return marshalResponse(validateXMLData(xmlText, xsdText))
}

func jsStringArgument(value js.Value, label string, maxBytes int64) (string, string) {
	if value.Type() != js.TypeString {
		return "", label + " must be a string"
	}
	boxed := js.Global().Get("Object").Invoke(value)
	if int64(boxed.Get("length").Int()) > maxBytes {
		return "", label + " exceeds " + byteLimit(maxBytes) + " limit"
	}
	text := value.String()
	if int64(len(text)) > maxBytes {
		return "", label + " exceeds " + byteLimit(maxBytes) + " limit"
	}
	return text, ""
}

func marshalResponse(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"status":"error","error":"internal serialization error"}`
	}
	return string(data)
}
