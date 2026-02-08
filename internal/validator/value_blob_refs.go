package validator

import "github.com/jacoelho/xsd/internal/runtime"

func valueBytes(values runtime.ValueBlob, ref runtime.ValueRef) []byte {
	if !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start := int(ref.Off)
	end := start + int(ref.Len)
	if start < 0 || end < 0 || end > len(values.Blob) {
		return nil
	}
	return values.Blob[start:end]
}

func valueKeyBytes(values runtime.ValueBlob, ref runtime.ValueKeyRef) []byte {
	if !ref.Ref.Present {
		return nil
	}
	return valueBytes(values, ref.Ref)
}
