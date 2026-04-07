package validator

import "github.com/jacoelho/xsd/internal/runtime"

// StoreRaw appends one raw validated attribute when storage is enabled.
func StoreRaw(
	validated []Start,
	attr Start,
	store bool,
	stabilizeName func(*Start),
	storeValue func([]byte) []byte,
) []Start {
	if !store {
		return validated
	}
	if stabilizeName != nil {
		stabilizeName(&attr)
	}
	if storeValue != nil {
		attr.Value = storeValue(attr.Value)
	}
	attr.KeyKind = runtime.VKInvalid
	attr.KeyBytes = nil
	return append(validated, attr)
}

// StoreCanonical appends one canonical validated attribute when storage is enabled.
func StoreCanonical(
	validated []Start,
	attr Start,
	store bool,
	stabilizeName func(*Start),
	canonical []byte,
	keyKind runtime.ValueKind,
	keyBytes []byte,
) []Start {
	if !store {
		return validated
	}
	if stabilizeName != nil {
		stabilizeName(&attr)
	}
	attr.Value = canonical
	attr.KeyKind = keyKind
	attr.KeyBytes = keyBytes
	return append(validated, attr)
}
