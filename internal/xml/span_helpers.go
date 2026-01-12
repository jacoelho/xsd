package xsdxml

import (
	"unsafe"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

type spanDecodeMode int

const (
	spanDecodeCopy spanDecodeMode = iota
	spanDecodeUnescape
)

func appendSpanString(dst []byte, span xmltext.Span, mode spanDecodeMode) ([]byte, string, error) {
	start := len(dst)
	if mode == spanDecodeUnescape {
		out, err := xmltext.UnescapeInto(dst, span)
		if err != nil {
			return dst[:start], "", err
		}
		dst = out
	} else {
		dst = xmltext.CopySpan(dst, span)
	}
	end := len(dst)
	if end == start {
		return dst, "", nil
	}
	return dst, unsafeString(dst[start:end]), nil
}

func unsafeString(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(data), len(data))
}
