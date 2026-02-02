package parser

import (
	"strconv"
	"strings"
)

const (
	importContextSeparator = "||"
	importContextPrefix    = "ic:"
)

// ImportContextKey combines a filesystem key and location into a stable lookup key.
func ImportContextKey(fsKey, location string) string {
	return importContextPrefix + strconv.Itoa(len(fsKey)) + ":" + fsKey + location
}

// ImportContextLocation extracts the original location from an import context key.
func ImportContextLocation(key string) string {
	if after, ok := strings.CutPrefix(key, importContextPrefix); ok {
		rest := after
		idx := strings.IndexByte(rest, ':')
		if idx == -1 {
			return key
		}
		size, err := strconv.Atoi(rest[:idx])
		if err != nil || size < 0 {
			return key
		}
		payload := rest[idx+1:]
		if size > len(payload) {
			return key
		}
		return payload[size:]
	}
	if idx := strings.Index(key, importContextSeparator); idx != -1 {
		return key[idx+len(importContextSeparator):]
	}
	return key
}
