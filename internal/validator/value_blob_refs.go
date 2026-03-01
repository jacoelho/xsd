package validator

import "github.com/jacoelho/xsd/internal/runtime"

func valueBytes(values runtime.ValueBlob, ref runtime.ValueRef) []byte {
	if !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start, end, ok := checkedSpan(ref.Off, ref.Len, len(values.Blob))
	if !ok {
		return nil
	}
	return values.Blob[start:end]
}
