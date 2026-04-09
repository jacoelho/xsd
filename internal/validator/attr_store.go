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

// StoreRawIdentity appends one raw validated attribute for identity processing
// without persisting the lexical value bytes.
func StoreRawIdentity(
	validated []Start,
	attr Start,
	store bool,
	stabilizeName func(*Start),
) []Start {
	if !store {
		return validated
	}
	stabilizeIdentityName(&attr, stabilizeName)
	attr.Value = nil
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

// StoreCanonicalIdentity appends one validated attribute for identity
// processing without retaining the canonical value bytes.
func StoreCanonicalIdentity(
	validated []Start,
	attr Start,
	store bool,
	stabilizeName func(*Start),
	keyKind runtime.ValueKind,
	keyBytes []byte,
) []Start {
	if !store {
		return validated
	}
	stabilizeIdentityName(&attr, stabilizeName)
	attr.Value = nil
	attr.KeyKind = keyKind
	attr.KeyBytes = keyBytes
	return append(validated, attr)
}

func stabilizeIdentityName(attr *Start, stabilizeName func(*Start)) {
	if attr == nil {
		return
	}
	if attr.Sym != 0 {
		attr.Local = nil
		attr.NSBytes = nil
		attr.NameCached = true
		return
	}
	if stabilizeName != nil {
		stabilizeName(attr)
	}
}
