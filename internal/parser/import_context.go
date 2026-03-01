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
		before, after0, ok0 := strings.Cut(rest, ":")
		if !ok0 {
			return key
		}
		size, err := strconv.Atoi(before)
		if err != nil || size < 0 {
			return key
		}
		payload := after0
		if size > len(payload) {
			return key
		}
		return payload[size:]
	}
	if _, after, ok := strings.Cut(key, importContextSeparator); ok {
		return after
	}
	return key
}
