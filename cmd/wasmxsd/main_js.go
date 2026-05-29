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
	return marshalResponse(formatXMLData(args[0].String()))
}

func validateXMLJS(this js.Value, args []js.Value) any {
	if len(args) != 2 {
		return marshalResponse(validateResponse{Error: "invalid number of arguments"})
	}
	return marshalResponse(validateXMLData(args[0].String(), args[1].String()))
}

func marshalResponse(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"internal serialization error"}`
	}
	return string(data)
}
