package valruntime

import (
	"encoding/base64"
	"errors"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/value"
)

// KindLoader resolves runtime string and integer validator kinds.
type KindLoader struct {
	StringKind  func(runtime.ValidatorMeta) (runtime.StringKind, bool)
	IntegerKind func(runtime.ValidatorMeta) (runtime.IntegerKind, bool)
}

// CanonicalBuffers carries caller-owned scratch slices between canonicalization calls.
type CanonicalBuffers struct {
	Buf1  []byte
	Buf2  []byte
	Value []byte
	Key   []byte
}

// CanonicalResult holds one canonicalized value and its optional derived runtime key.
type CanonicalResult struct {
	Canonical []byte
	Key       []byte
	KeyKind   runtime.ValueKind
}

// HasKey reports whether a derived runtime key is present.
func (r CanonicalResult) HasKey() bool {
	return r.KeyKind != runtime.VKInvalid
}

// Atomic canonicalizes one atomic primitive family value.
func Atomic(meta runtime.ValidatorMeta, normalized []byte, needKey bool, kinds KindLoader, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	switch meta.Kind {
	case runtime.VString:
		return atomicString(meta, normalized, needKey, kinds, bufs)
	case runtime.VBoolean:
		return atomicBoolean(normalized, needKey, bufs)
	case runtime.VDecimal:
		return atomicDecimal(normalized, needKey, bufs, cache)
	case runtime.VInteger:
		return atomicInteger(meta, normalized, needKey, kinds, bufs, cache)
	case runtime.VFloat:
		return atomicFloat(normalized, needKey, bufs, cache)
	case runtime.VDouble:
		return atomicDouble(normalized, needKey, bufs, cache)
	case runtime.VDuration:
		return atomicDuration(normalized, needKey, bufs)
	default:
		return CanonicalResult{}, bufs, diag.Invalidf("unsupported atomic kind %d", meta.Kind)
	}
}

// Temporal canonicalizes one temporal primitive value.
func Temporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, bufs CanonicalBuffers) (CanonicalResult, CanonicalBuffers, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return CanonicalResult{}, bufs, diag.Invalidf("unsupported temporal kind %d", kind)
	}
	tv, err := value.Parse(spec.Kind, normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}

	result := CanonicalResult{
		Canonical: []byte(value.Canonical(tv)),
	}
	if needKey {
		key := runtime.TemporalKeyBytes(bufs.Key[:0], spec.KeyTag, tv.Time, tv.TimezoneKind, tv.LeapSecond)
		bufs.Key = key
		result.KeyKind = runtime.VKDateTime
		result.Key = key
	}
	return result, bufs, nil
}

// AnyURI canonicalizes one xs:anyURI value.
func AnyURI(normalized []byte, needKey bool, bufs CanonicalBuffers) (CanonicalResult, CanonicalBuffers, error) {
	if err := value.ValidateAnyURI(normalized); err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}

	result := CanonicalResult{Canonical: normalized}
	if needKey {
		key := runtime.StringKeyBytes(bufs.Key[:0], 1, normalized)
		bufs.Key = key
		result.KeyKind = runtime.VKString
		result.Key = key
	}
	return result, bufs, nil
}

// QName canonicalizes one xs:QName or xs:NOTATION value.
func QName(kind runtime.ValidatorKind, normalized []byte, resolver value.NSResolver, needKey bool, notationDeclared func([]byte) bool, bufs CanonicalBuffers) (CanonicalResult, CanonicalBuffers, error) {
	canon, err := value.CanonicalQName(normalized, resolver, bufs.Value[:0])
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}
	bufs.Value = canon

	if kind == runtime.VNotation && (notationDeclared == nil || !notationDeclared(canon)) {
		return CanonicalResult{}, bufs, diag.Invalid("notation not declared")
	}

	result := CanonicalResult{Canonical: canon}
	if needKey {
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := runtime.QNameKeyCanonical(bufs.Key[:0], tag, canon)
		if len(key) == 0 {
			return CanonicalResult{}, bufs, diag.Invalid("invalid QName key")
		}
		bufs.Key = key
		result.KeyKind = runtime.VKQName
		result.Key = key
	}
	return result, bufs, nil
}

// HexBinary canonicalizes one xs:hexBinary value.
func HexBinary(normalized []byte, needKey bool, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	decoded, err := value.ParseHexBinary(normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}
	cache.SetLength(len(decoded))

	canon := value.UpperHex(bufs.Value[:0], decoded)
	bufs.Value = canon
	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := runtime.BinaryKeyBytes(bufs.Key[:0], 0, decoded)
		bufs.Key = key
		result.KeyKind = runtime.VKBinary
		result.Key = key
	}
	return result, bufs, nil
}

// Base64Binary canonicalizes one xs:base64Binary value.
func Base64Binary(normalized []byte, needKey bool, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	decoded, err := value.ParseBase64Binary(normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}
	cache.SetLength(len(decoded))

	canonLen := base64.StdEncoding.EncodedLen(len(decoded))
	canon := bufs.Value[:0]
	if cap(canon) < canonLen {
		canon = make([]byte, canonLen)
	} else {
		canon = canon[:canonLen]
	}
	base64.StdEncoding.Encode(canon, decoded)
	bufs.Value = canon

	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := runtime.BinaryKeyBytes(bufs.Key[:0], 1, decoded)
		bufs.Key = key
		result.KeyKind = runtime.VKBinary
		result.Key = key
	}
	return result, bufs, nil
}

// SplitQName splits one canonical QName into namespace URI and local name.
func SplitQName(canonical []byte) ([]byte, []byte, error) {
	for i, b := range canonical {
		if b == 0 {
			return canonical[:i], canonical[i+1:], nil
		}
	}
	return nil, nil, errors.New("invalid canonical QName")
}

func atomicString(meta runtime.ValidatorMeta, normalized []byte, needKey bool, kinds KindLoader, bufs CanonicalBuffers) (CanonicalResult, CanonicalBuffers, error) {
	if kinds.StringKind == nil {
		return CanonicalResult{}, bufs, diag.Invalid("string validator out of range")
	}
	kind, ok := kinds.StringKind(meta)
	if !ok {
		return CanonicalResult{}, bufs, diag.Invalid("string validator out of range")
	}
	if err := runtime.ValidateStringKind(kind, normalized); err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}

	result := CanonicalResult{Canonical: normalized}
	if needKey {
		key := runtime.StringKeyBytes(bufs.Key[:0], 0, normalized)
		bufs.Key = key
		result.KeyKind = runtime.VKString
		result.Key = key
	}
	return result, bufs, nil
}

func atomicBoolean(normalized []byte, needKey bool, bufs CanonicalBuffers) (CanonicalResult, CanonicalBuffers, error) {
	v, canon, err := value.CanonicalizeBoolean(normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}

	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := bufs.Key[:0]
		key = append(key, 0)
		if v {
			key[0] = 1
		}
		bufs.Key = key
		result.KeyKind = runtime.VKBool
		result.Key = key
	}
	return result, bufs, nil
}

func atomicDecimal(normalized []byte, needKey bool, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	dec, buf, perr := num.ParseDecInto(normalized, bufs.Buf1)
	if perr != nil {
		return CanonicalResult{}, bufs, diag.Invalid("invalid decimal")
	}
	bufs.Buf1 = buf
	cache.SetDecimal(dec)

	canon := dec.RenderCanonical(bufs.Buf2[:0])
	bufs.Buf2 = canon
	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := num.EncodeDecKey(bufs.Key[:0], dec)
		bufs.Key = key
		result.KeyKind = runtime.VKDecimal
		result.Key = key
	}
	return result, bufs, nil
}

func atomicInteger(meta runtime.ValidatorMeta, normalized []byte, needKey bool, kinds KindLoader, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	if kinds.IntegerKind == nil {
		return CanonicalResult{}, bufs, diag.Invalid("integer validator out of range")
	}
	kind, ok := kinds.IntegerKind(meta)
	if !ok {
		return CanonicalResult{}, bufs, diag.Invalid("integer validator out of range")
	}

	intVal, perr := num.ParseInt(normalized)
	if perr != nil {
		return CanonicalResult{}, bufs, diag.Invalid("invalid integer")
	}
	if err := runtime.ValidateIntegerKind(kind, intVal); err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}
	cache.SetInteger(intVal)

	canon := intVal.RenderCanonical(bufs.Buf2[:0])
	bufs.Buf2 = canon
	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := num.EncodeDecKey(bufs.Key[:0], intVal.AsDec())
		bufs.Key = key
		result.KeyKind = runtime.VKDecimal
		result.Key = key
	}
	return result, bufs, nil
}

func atomicFloat(normalized []byte, needKey bool, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	v, class, canon, err := value.CanonicalizeFloat32(normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}
	cache.SetFloat32(v, class)

	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := runtime.Float32Key(bufs.Key[:0], v, class)
		bufs.Key = key
		result.KeyKind = runtime.VKFloat32
		result.Key = key
	}
	return result, bufs, nil
}

func atomicDouble(normalized []byte, needKey bool, bufs CanonicalBuffers, cache *Cache) (CanonicalResult, CanonicalBuffers, error) {
	v, class, canon, err := value.CanonicalizeFloat64(normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}
	cache.SetFloat64(v, class)

	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := runtime.Float64Key(bufs.Key[:0], v, class)
		bufs.Key = key
		result.KeyKind = runtime.VKFloat64
		result.Key = key
	}
	return result, bufs, nil
}

func atomicDuration(normalized []byte, needKey bool, bufs CanonicalBuffers) (CanonicalResult, CanonicalBuffers, error) {
	dur, canon, err := value.CanonicalizeDuration(normalized)
	if err != nil {
		return CanonicalResult{}, bufs, diag.Invalid(err.Error())
	}

	result := CanonicalResult{Canonical: canon}
	if needKey {
		key := runtime.DurationKeyBytes(bufs.Key[:0], dur)
		bufs.Key = key
		result.KeyKind = runtime.VKDuration
		result.Key = key
	}
	return result, bufs, nil
}
