//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
)

var jsFuncs []js.Func

func main() {
	holdJSFunc("formatXML", formatXMLJS)
	holdJSFunc("validateXML", validateXMLJS)
	js.Global().Set("xsdLimits", map[string]any{
		"maxXMLBytes": maxXMLBytes,
		"maxXSDBytes": maxXSDBytes,
	})
	select {}
}

func holdJSFunc(name string, fn func(js.Value, []js.Value) any) {
	f := js.FuncOf(fn)
	jsFuncs = append(jsFuncs, f)
	js.Global().Set(name, f)
}

func formatXMLJS(this js.Value, args []js.Value) any {
	if len(args) != 1 {
		return marshalResponse(formatResponse{Error: "invalid number of arguments"})
	}
	input, inputErr := jsStringArgument(args[0], "XML", maxXMLBytes)
	if inputErr != "" {
		return marshalResponse(formatResponse{Error: inputErr})
	}
	return marshalResponse(formatXMLData(input))
}

func validateXMLJS(this js.Value, args []js.Value) any {
	if len(args) != 2 {
		return marshalResponse(validateResponse{Error: "invalid number of arguments"})
	}
	xmlText, xmlErr := jsStringArgument(args[0], "XML", maxXMLBytes)
	if xmlErr != "" {
		return marshalResponse(validateResponse{Error: xmlErr})
	}
	xsdText, xsdErr := jsStringArgument(args[1], "XSD", maxXSDBytes)
	if xsdErr != "" {
		return marshalResponse(validateResponse{Error: xsdErr})
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
	return value.String(), ""
}

func marshalResponse(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"internal serialization error"}`
	}
	return string(data)
}
