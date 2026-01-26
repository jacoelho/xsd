package parser

import "strings"

const importContextSeparator = "||"

// ImportContextKey combines a filesystem key and location into a stable lookup key.
func ImportContextKey(fsKey, location string) string {
	if fsKey == "" {
		return location
	}
	return fsKey + importContextSeparator + location
}

// ImportContextLocation extracts the original location from an import context key.
func ImportContextLocation(key string) string {
	if idx := strings.Index(key, importContextSeparator); idx != -1 {
		return key[idx+len(importContextSeparator):]
	}
	return key
}
