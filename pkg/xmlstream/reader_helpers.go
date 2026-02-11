package xmlstream

import (
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func resolveElementParts(ns *nsStack, dec *xmltext.Decoder, name []byte, nameColon, depth, line, column int) (string, []byte, error) {
	prefix, local, hasPrefix := splitQNameWithColon(name, nameColon)
	if hasPrefix {
		prefixName := unsafeString(prefix)
		namespace, ok := ns.lookup(prefixName, depth)
		if !ok {
			return "", nil, unboundPrefixError(dec, line, column)
		}
		return namespace, local, nil
	}
	namespace, _ := ns.lookup("", depth)
	return namespace, local, nil
}

func popQName(stack []QName, depth int) (QName, []QName, error) {
	if len(stack) == 0 {
		return QName{}, nil, fmt.Errorf("unexpected end element at depth %d", depth)
	}
	name := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	return name, stack, nil
}

func decodeAttrValueBytes(dec *xmltext.Decoder, buf, value []byte) ([]byte, []byte, error) {
	start := len(buf)
	next, err := unescapeIntoBuffer(dec, buf, start, value)
	if err != nil {
		if len(next) >= start {
			next = next[:start]
		}
		return next, nil, err
	}
	if len(next) == start {
		return next, nil, nil
	}
	return next, next[start:], nil
}

func decodeNamespaceValueString(dec *xmltext.Decoder, buf, value []byte) ([]byte, string, error) {
	start := len(buf)
	next, err := unescapeIntoBuffer(dec, buf, start, value)
	if err != nil {
		if len(next) >= start {
			next = next[:start]
		}
		return next, "", err
	}
	if len(next) == start {
		return next, "", nil
	}
	return next, unsafeString(next[start:]), nil
}

func appendNamespaceValue(buf, value []byte) ([]byte, string) {
	start := len(buf)
	buf = append(buf, value...)
	if len(buf) == start {
		return buf, ""
	}
	return buf, unsafeString(buf[start:])
}

func decodeTextBytes(dec *xmltext.Decoder, buf, text []byte) ([]byte, []byte, error) {
	start := len(buf)
	next, err := unescapeIntoBuffer(dec, buf, start, text)
	if err != nil {
		if len(next) >= start {
			next = next[:start]
		}
		return next, nil, err
	}
	if len(next) == start {
		return next, nil, nil
	}
	return next, next[start:], nil
}
