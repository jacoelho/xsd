package validator

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
)

type valueErrorKind uint8

const (
	valueErrInvalid valueErrorKind = iota
	valueErrFacet
)

type valueError struct {
	msg  string
	kind valueErrorKind
}

func (e valueError) Error() string { return e.msg }

func valueErrorf(kind valueErrorKind, format string, args ...any) error {
	return valueError{kind: kind, msg: fmt.Sprintf(format, args...)}
}

func valueErrorMsg(kind valueErrorKind, msg string) error {
	return valueError{kind: kind, msg: msg}
}

func valueErrorKindOf(err error) (valueErrorKind, bool) {
	if err == nil {
		return 0, false
	}
	var ve valueError
	if errors.As(err, &ve) {
		return ve.kind, true
	}
	return 0, false
}

var (
	intZero   = num.Int{Sign: 0, Digits: []byte{'0'}}
	minInt8   = num.Int{Sign: -1, Digits: []byte("128")}
	maxInt8   = num.Int{Sign: 1, Digits: []byte("127")}
	minInt16  = num.Int{Sign: -1, Digits: []byte("32768")}
	maxInt16  = num.Int{Sign: 1, Digits: []byte("32767")}
	minInt32  = num.Int{Sign: -1, Digits: []byte("2147483648")}
	maxInt32  = num.Int{Sign: 1, Digits: []byte("2147483647")}
	minInt64  = num.Int{Sign: -1, Digits: []byte("9223372036854775808")}
	maxInt64  = num.Int{Sign: 1, Digits: []byte("9223372036854775807")}
	maxUint8  = num.Int{Sign: 1, Digits: []byte("255")}
	maxUint16 = num.Int{Sign: 1, Digits: []byte("65535")}
	maxUint32 = num.Int{Sign: 1, Digits: []byte("4294967295")}
	maxUint64 = num.Int{Sign: 1, Digits: []byte("18446744073709551615")}
)

type valueMetrics struct {
	keyBytes       []byte
	fractionDigits int
	totalDigits    int
	listCount      int
	length         int
	keyKind        runtime.ValueKind
	lengthSet      bool
	digitsSet      bool
	listSet        bool
	keySet         bool
	patternChecked bool
	enumChecked    bool
	actualTypeID   runtime.TypeID

	decSet     bool
	intSet     bool
	float32Set bool
	float64Set bool

	decVal       num.Dec
	intVal       num.Int
	float32Val   float32
	float32Class num.FloatClass
	float64Val   float64
	float64Class num.FloatClass
}

type valueOptions struct {
	applyWhitespace  bool
	trackIDs         bool
	requireCanonical bool
	storeValue       bool
	needKey          bool
}

func (s *Session) setKey(metrics *valueMetrics, kind runtime.ValueKind, key []byte, store bool) {
	if s == nil || metrics == nil {
		return
	}
	metrics.keyKind = kind
	if store {
		metrics.keyBytes = s.storeKey(key)
	} else {
		metrics.keyBytes = key
	}
	metrics.keySet = true
}

func (s *Session) storeValue(data []byte) []byte {
	if s == nil {
		return nil
	}
	start := len(s.valueBuf)
	s.valueBuf = append(s.valueBuf, data...)
	return s.valueBuf[start:len(s.valueBuf)]
}

func (s *Session) maybeStore(data []byte, store bool) []byte {
	if store {
		return s.storeValue(data)
	}
	return data
}

func (s *Session) storeKey(data []byte) []byte {
	if s == nil {
		return nil
	}
	start := len(s.keyBuf)
	s.keyBuf = append(s.keyBuf, data...)
	return s.keyBuf[start:len(s.keyBuf)]
}

func (s *Session) validateValueInternalNoTrack(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, applyWhitespace bool) ([]byte, error) {
	return s.validateValueInternalOptions(id, lexical, resolver, valueOptions{
		applyWhitespace:  applyWhitespace,
		trackIDs:         false,
		requireCanonical: true,
		storeValue:       true,
	})
}

func (s *Session) validateValueInternalOptions(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	return s.validateValueCore(id, lexical, resolver, opts, nil)
}

func (s *Session) validateValueInternalWithMetrics(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, valueMetrics, error) {
	var metrics valueMetrics
	canon, err := s.validateValueCore(id, lexical, resolver, opts, &metrics)
	return canon, metrics, err
}

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metrics *valueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return nil, valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	normalized := lexical
	if opts.applyWhitespace && meta.Kind != runtime.VUnion {
		normalized = value.NormalizeWhitespace(meta.WhiteSpace, lexical, s.normBuf)
	}
	needsCanonical := opts.requireCanonical || meta.Facets.Len != 0 || meta.Kind == runtime.VUnion || meta.Kind == runtime.VQName || meta.Kind == runtime.VNotation
	if opts.storeValue || opts.needKey {
		needsCanonical = true
	}
	needEnumKey := meta.Flags&runtime.ValidatorHasEnum != 0
	if metrics == nil && needEnumKey {
		var localMetrics valueMetrics
		metrics = &localMetrics
	}
	// for atomic types, keys can be computed lazily in applyFacets when metrics is nil
	needKey := opts.needKey || opts.storeValue || needEnumKey
	if !needsCanonical {
		return s.validateValueNoCanonical(meta, normalized, resolver, opts)
	}
	canon, err := s.canonicalizeValueCore(meta, normalized, resolver, opts, needKey, metrics)
	if err != nil {
		return nil, err
	}
	if err := s.applyFacets(meta, normalized, canon, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) canonicalizeValueCore(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *valueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "string validator out of range")
		}
		if err := validateStringKind(kind, normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canon := s.maybeStore(normalized, opts.storeValue)
		if opts.trackIDs {
			if err := s.trackIDs(kind, canon); err != nil {
				return nil, err
			}
		}
		if needKey {
			key := stringKeyBytes(s.keyTmp[:0], 0, canon)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKString, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VBoolean:
		v, err := value.ParseBoolean(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonRaw := []byte("false")
		if v {
			canonRaw = []byte("true")
		}
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := byte(0)
			if v {
				key = 1
			}
			s.setKey(metrics, runtime.VKBool, []byte{key}, opts.storeValue)
		}
		return canon, nil
	case runtime.VDecimal:
		dec, buf, perr := num.ParseDecInto(normalized, s.Scratch.Buf1)
		if perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		s.Scratch.Buf1 = buf
		if metrics != nil {
			metrics.decVal = dec
			metrics.decSet = true
			metrics.totalDigits = len(dec.Coef)
			metrics.fractionDigits = int(dec.Scale)
			metrics.digitsSet = true
		}
		canonRaw := dec.RenderCanonical(s.Scratch.Buf2[:0])
		s.Scratch.Buf2 = canonRaw
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := num.EncodeDecKey(s.keyTmp[:0], dec)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDecimal, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VInteger:
		kind, ok := s.integerKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "integer validator out of range")
		}
		intVal, perr := num.ParseInt(normalized)
		if perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		if err := validateIntegerKind(kind, intVal); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if metrics != nil {
			metrics.intVal = intVal
			metrics.intSet = true
			metrics.totalDigits = len(intVal.Digits)
			metrics.fractionDigits = 0
			metrics.digitsSet = true
		}
		canonRaw := intVal.RenderCanonical(s.Scratch.Buf2[:0])
		s.Scratch.Buf2 = canonRaw
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := num.EncodeIntKey(s.keyTmp[:0], intVal)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDecimal, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VFloat:
		v, class, perr := num.ParseFloat32(normalized)
		if perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid float")
		}
		if metrics != nil {
			metrics.float32Val = v
			metrics.float32Class = class
			metrics.float32Set = true
		}
		canonRaw := []byte(value.CanonicalFloat(float64(v), 32))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := float32KeyBytes(s.keyTmp[:0], v, class)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKFloat32, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VDouble:
		v, class, perr := num.ParseFloat64(normalized)
		if perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid double")
		}
		if metrics != nil {
			metrics.float64Val = v
			metrics.float64Class = class
			metrics.float64Set = true
		}
		canonRaw := []byte(value.CanonicalFloat(v, 64))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := float64KeyBytes(s.keyTmp[:0], v, class)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKFloat64, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VDuration:
		dur, err := types.ParseXSDDuration(string(normalized))
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonRaw := []byte(types.ComparableXSDDuration{Value: dur}.String())
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := durationKeyBytes(s.keyTmp[:0], dur)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDuration, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VDateTime:
		t, err := value.ParseDateTime(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "dateTime", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 0, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VTime:
		t, err := value.ParseTime(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "time", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 2, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VDate:
		t, err := value.ParseDate(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "date", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 1, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VGYearMonth:
		t, err := value.ParseGYearMonth(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "gYearMonth", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 3, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VGYear:
		t, err := value.ParseGYear(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "gYear", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 4, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VGMonthDay:
		t, err := value.ParseGMonthDay(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "gMonthDay", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 5, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VGDay:
		t, err := value.ParseGDay(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "gDay", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 6, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VGMonth:
		t, err := value.ParseGMonth(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(normalized)
		canonRaw := []byte(value.CanonicalDateTimeString(t, "gMonth", hasTZ))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := temporalKeyBytes(s.keyTmp[:0], 7, t, hasTZ)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDateTime, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VAnyURI:
		if err := value.ValidateAnyURI(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canon := s.maybeStore(normalized, opts.storeValue)
		if needKey {
			key := stringKeyBytes(s.keyTmp[:0], 1, canon)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKString, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VQName, runtime.VNotation:
		canon, err := value.CanonicalQName(normalized, resolver, nil)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonStored := s.maybeStore(canon, opts.storeValue)
		if needKey {
			tag := byte(0)
			if meta.Kind == runtime.VNotation {
				tag = 1
			}
			key := qnameKeyBytes(s.keyTmp[:0], tag, canonStored)
			if len(key) == 0 {
				return nil, valueErrorf(valueErrInvalid, "invalid QName key")
			}
			s.keyTmp = key
			s.setKey(metrics, runtime.VKQName, key, opts.storeValue)
		}
		return canonStored, nil
	case runtime.VHexBinary:
		decoded, err := types.ParseHexBinary(string(normalized))
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if metrics != nil {
			metrics.length = len(decoded)
			metrics.lengthSet = true
		}
		canonRaw := []byte(strings.ToUpper(fmt.Sprintf("%x", decoded)))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := binaryKeyBytes(s.keyTmp[:0], 0, decoded)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKBinary, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VBase64Binary:
		decoded, err := types.ParseBase64Binary(string(normalized))
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if metrics != nil {
			metrics.length = len(decoded)
			metrics.lengthSet = true
		}
		canonRaw := []byte(encodeBase64(decoded))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := binaryKeyBytes(s.keyTmp[:0], 1, decoded)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKBinary, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VList:
		itemValidator, ok := s.listItemValidator(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "list validator out of range")
		}
		count := 0
		tmp := make([]byte, 0, len(normalized))
		var keyTmp []byte
		if needKey {
			keyTmp = make([]byte, 0, len(normalized))
		}
		spaceOnly := opts.applyWhitespace && meta.WhiteSpace == runtime.WS_Collapse
		_, err := forEachListItem(normalized, spaceOnly, func(item []byte) error {
			itemOpts := opts
			itemOpts.applyWhitespace = false
			itemOpts.requireCanonical = true
			itemOpts.storeValue = false
			itemOpts.needKey = needKey
			canon, itemMetrics, err := s.validateValueInternalWithMetrics(itemValidator, item, resolver, itemOpts)
			if err != nil {
				return err
			}
			if count > 0 {
				tmp = append(tmp, ' ')
			}
			tmp = append(tmp, canon...)
			if needKey {
				if !itemMetrics.keySet {
					return valueErrorf(valueErrInvalid, "list item key missing")
				}
				keyTmp = runtime.AppendListKey(keyTmp, itemMetrics.keyKind, itemMetrics.keyBytes)
			}
			count++
			return nil
		})
		if err != nil {
			return nil, err
		}
		if metrics != nil {
			metrics.listCount = count
			metrics.listSet = true
		}
		canonRaw := tmp
		if count == 0 {
			canonRaw = []byte{}
		}
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			listKey := s.keyTmp[:0]
			listKey = appendUvarint(listKey, uint64(count))
			listKey = append(listKey, keyTmp...)
			s.keyTmp = listKey
			s.setKey(metrics, runtime.VKList, listKey, opts.storeValue)
		}
		return canon, nil
	case runtime.VUnion:
		memberValidators, memberTypes, ok := s.unionMemberInfo(meta)
		if !ok || len(memberValidators) == 0 {
			return nil, valueErrorf(valueErrInvalid, "union validator out of range")
		}
		facets, err := s.facetProgram(meta)
		if err != nil {
			return nil, err
		}
		enumIDs := collectEnumIDs(facets)
		patternChecked, err := s.checkUnionPatterns(facets, normalized)
		if err != nil {
			return nil, err
		}
		if metrics != nil {
			metrics.patternChecked = patternChecked
		}
		sawValid := false
		var lastErr error
		for i, member := range memberValidators {
			memberOpts := opts
			memberOpts.applyWhitespace = true
			memberOpts.requireCanonical = true
			memberOpts.storeValue = false
			memberOpts.needKey = needKey
			canon, memberMetrics, err := s.validateValueInternalWithMetrics(member, normalized, resolver, memberOpts)
			if err != nil {
				lastErr = err
				continue
			}
			sawValid = true
			if len(enumIDs) > 0 && !s.enumSetsContain(enumIDs, memberMetrics.keyKind, memberMetrics.keyBytes) {
				continue
			}
			if metrics != nil {
				metrics.keyKind = memberMetrics.keyKind
				metrics.keyBytes = memberMetrics.keyBytes
				metrics.keySet = memberMetrics.keySet
				if len(enumIDs) > 0 {
					metrics.enumChecked = true
				}
				if i < len(memberTypes) {
					metrics.actualTypeID = memberTypes[i]
				}
				if opts.storeValue && metrics.keySet {
					s.setKey(metrics, metrics.keyKind, metrics.keyBytes, true)
				}
			}
			return canon, nil
		}
		if sawValid && len(enumIDs) > 0 {
			return nil, valueErrorf(valueErrFacet, "enumeration violation")
		}
		if lastErr == nil {
			lastErr = valueErrorf(valueErrInvalid, "union value does not match any member type")
		}
		return nil, lastErr
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
	}
}

func (s *Session) validateValueNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "string validator out of range")
		}
		if err := validateStringKind(kind, normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canon := s.maybeStore(normalized, opts.storeValue)
		if opts.trackIDs {
			if err := s.trackIDs(kind, canon); err != nil {
				return nil, err
			}
		}
		return canon, nil
	case runtime.VBoolean:
		if _, err := value.ParseBoolean(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VDecimal:
		if _, perr := num.ParseDec(normalized); perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VInteger:
		kind, ok := s.integerKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "integer validator out of range")
		}
		intVal, perr := num.ParseInt(normalized)
		if perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		if err := validateIntegerKind(kind, intVal); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VFloat:
		if perr := num.ValidateFloatLexical(normalized); perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid float")
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VDouble:
		if perr := num.ValidateFloatLexical(normalized); perr != nil {
			return nil, valueErrorMsg(valueErrInvalid, "invalid double")
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VDuration:
		if _, err := types.ParseXSDDuration(string(normalized)); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VDateTime:
		if _, err := value.ParseDateTime(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VTime:
		if _, err := value.ParseTime(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VDate:
		if _, err := value.ParseDate(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VGYearMonth:
		if _, err := value.ParseGYearMonth(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VGYear:
		if _, err := value.ParseGYear(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VGMonthDay:
		if _, err := value.ParseGMonthDay(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VGDay:
		if _, err := value.ParseGDay(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VGMonth:
		if _, err := value.ParseGMonth(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VAnyURI:
		if err := value.ValidateAnyURI(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VHexBinary:
		if _, err := types.ParseHexBinary(string(normalized)); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VBase64Binary:
		if _, err := types.ParseBase64Binary(string(normalized)); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VList:
		itemValidator, ok := s.listItemValidator(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "list validator out of range")
		}
		spaceOnly := opts.applyWhitespace && meta.WhiteSpace == runtime.WS_Collapse
		if _, err := forEachListItem(normalized, spaceOnly, func(item []byte) error {
			itemOpts := opts
			itemOpts.applyWhitespace = false
			itemOpts.requireCanonical = false
			itemOpts.storeValue = false
			if _, err := s.validateValueInternalOptions(itemValidator, item, resolver, itemOpts); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
	}
}

func (s *Session) applyFacets(meta runtime.ValidatorMeta, normalized, canonical []byte, metrics *valueMetrics) error {
	if s == nil || s.rt == nil {
		return valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if meta.Facets.Len == 0 {
		return nil
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(s.rt.Facets) {
		return valueErrorf(valueErrInvalid, "facet program out of range")
	}
	for _, instr := range s.rt.Facets[start:end] {
		switch instr.Op {
		case runtime.FPattern:
			if metrics != nil && metrics.patternChecked {
				continue
			}
			if int(instr.Arg0) >= len(s.rt.Patterns) {
				return valueErrorf(valueErrInvalid, "pattern %d out of range", instr.Arg0)
			}
			pat := s.rt.Patterns[instr.Arg0]
			if pat.Re != nil && !pat.Re.Match(normalized) {
				return valueErrorf(valueErrFacet, "pattern violation")
			}
		case runtime.FEnum:
			if metrics != nil && metrics.enumChecked {
				continue
			}
			enumID := runtime.EnumID(instr.Arg0)
			// compute key lazily if not already set
			if metrics == nil || !metrics.keySet {
				kind, key, err := s.deriveKeyFromCanonical(meta.Kind, canonical)
				if err != nil {
					return err
				}
				if metrics != nil {
					metrics.keyKind = kind
					metrics.keyBytes = key
					metrics.keySet = true
				}
				if !runtime.EnumContains(&s.rt.Enums, enumID, kind, key) {
					return valueErrorf(valueErrFacet, "enumeration violation")
				}
			} else if !runtime.EnumContains(&s.rt.Enums, enumID, metrics.keyKind, metrics.keyBytes) {
				return valueErrorf(valueErrFacet, "enumeration violation")
			}
		case runtime.FMinInclusive, runtime.FMaxInclusive, runtime.FMinExclusive, runtime.FMaxExclusive:
			ref := runtime.ValueRef{Off: instr.Arg0, Len: instr.Arg1, Present: true}
			bound := valueBytes(s.rt.Values, ref)
			if bound == nil {
				return valueErrorf(valueErrInvalid, "range facet bound out of range")
			}
			switch meta.Kind {
			case runtime.VFloat:
				if err := s.checkFloat32Range(instr.Op, canonical, bound, metrics); err != nil {
					return err
				}
			case runtime.VDouble:
				if err := s.checkFloat64Range(instr.Op, canonical, bound, metrics); err != nil {
					return err
				}
			default:
				cmp, err := s.compareValue(meta.Kind, canonical, bound, metrics)
				if err != nil {
					return err
				}
				switch instr.Op {
				case runtime.FMinInclusive:
					if cmp < 0 {
						return valueErrorf(valueErrFacet, "minInclusive violation")
					}
				case runtime.FMaxInclusive:
					if cmp > 0 {
						return valueErrorf(valueErrFacet, "maxInclusive violation")
					}
				case runtime.FMinExclusive:
					if cmp <= 0 {
						return valueErrorf(valueErrFacet, "minExclusive violation")
					}
				case runtime.FMaxExclusive:
					if cmp >= 0 {
						return valueErrorf(valueErrFacet, "maxExclusive violation")
					}
				}
			}
		case runtime.FLength, runtime.FMinLength, runtime.FMaxLength:
			if shouldSkipRuntimeLengthFacet(meta.Kind) {
				continue
			}
			length := s.valueLength(meta, normalized, metrics)
			switch instr.Op {
			case runtime.FLength:
				if length != int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "length violation")
				}
			case runtime.FMinLength:
				if length < int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "minLength violation")
				}
			case runtime.FMaxLength:
				if length > int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "maxLength violation")
				}
			}
		case runtime.FTotalDigits, runtime.FFractionDigits:
			total, fraction, err := digitCounts(meta.Kind, canonical, metrics)
			if err != nil {
				return err
			}
			switch instr.Op {
			case runtime.FTotalDigits:
				if total > int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "totalDigits violation")
				}
			case runtime.FFractionDigits:
				if fraction > int(instr.Arg0) {
					return valueErrorf(valueErrFacet, "fractionDigits violation")
				}
			}
		default:
			return valueErrorf(valueErrInvalid, "unknown facet op %d", instr.Op)
		}
	}
	return nil
}

func (s *Session) facetProgram(meta runtime.ValidatorMeta) ([]runtime.FacetInstr, error) {
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if meta.Facets.Len == 0 {
		return nil, nil
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(s.rt.Facets) {
		return nil, valueErrorf(valueErrInvalid, "facet program out of range")
	}
	return s.rt.Facets[start:end], nil
}

func collectEnumIDs(facets []runtime.FacetInstr) []runtime.EnumID {
	if len(facets) == 0 {
		return nil
	}
	out := make([]runtime.EnumID, 0, len(facets))
	for _, instr := range facets {
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Session) checkUnionPatterns(facets []runtime.FacetInstr, normalized []byte) (bool, error) {
	if len(facets) == 0 {
		return false, nil
	}
	seen := false
	for _, instr := range facets {
		if instr.Op != runtime.FPattern {
			continue
		}
		seen = true
		if int(instr.Arg0) >= len(s.rt.Patterns) {
			return seen, valueErrorf(valueErrInvalid, "pattern %d out of range", instr.Arg0)
		}
		pat := s.rt.Patterns[instr.Arg0]
		if pat.Re != nil && !pat.Re.Match(normalized) {
			return seen, valueErrorf(valueErrFacet, "pattern violation")
		}
	}
	return seen, nil
}

func (s *Session) enumSetsContain(enumIDs []runtime.EnumID, keyKind runtime.ValueKind, keyBytes []byte) bool {
	if s == nil || s.rt == nil || len(enumIDs) == 0 {
		return false
	}
	for _, enumID := range enumIDs {
		if !runtime.EnumContains(&s.rt.Enums, enumID, keyKind, keyBytes) {
			return false
		}
	}
	return true
}

func (s *Session) valueLength(meta runtime.ValidatorMeta, normalized []byte, metrics *valueMetrics) int {
	if metrics != nil && metrics.lengthSet {
		return metrics.length
	}
	switch meta.Kind {
	case runtime.VList:
		if metrics != nil && metrics.listSet {
			metrics.length = metrics.listCount
			metrics.lengthSet = true
			return metrics.length
		}
		count := listItemCount(normalized, meta.WhiteSpace == runtime.WS_Collapse)
		if metrics != nil {
			metrics.length = count
			metrics.lengthSet = true
		}
		return count
	case runtime.VHexBinary, runtime.VBase64Binary:
		if metrics != nil && metrics.lengthSet {
			return metrics.length
		}
		return utf8.RuneCount(normalized)
	default:
		return utf8.RuneCount(normalized)
	}
}

func (s *Session) compareValue(kind runtime.ValidatorKind, canonical, bound []byte, metrics *valueMetrics) (int, error) {
	switch kind {
	case runtime.VDecimal:
		val, err := s.decForCanonical(canonical, metrics)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseDec(bound)
		if perr != nil {
			return 0, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		return val.Compare(boundVal), nil
	case runtime.VInteger:
		val, err := s.intForCanonical(canonical, metrics)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseInt(bound)
		if perr != nil {
			return 0, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		return val.Compare(boundVal), nil
	case runtime.VDuration:
		val, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		boundVal, err := types.ParseXSDDuration(string(bound))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		cmp, err := types.ComparableXSDDuration{Value: val}.Compare(types.ComparableXSDDuration{Value: boundVal})
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		valTime, valHasTZ, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return 0, err
		}
		boundTime, boundHasTZ, err := parseTemporalForKind(kind, bound)
		if err != nil {
			return 0, err
		}
		comp := types.ComparableTime{Value: valTime, HasTimezone: valHasTZ}
		boundComp := types.ComparableTime{Value: boundTime, HasTimezone: boundHasTZ}
		cmp, err := comp.Compare(boundComp)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	default:
		return 0, valueErrorf(valueErrInvalid, "unsupported comparable type %d", kind)
	}
}

func digitCounts(kind runtime.ValidatorKind, canonical []byte, metrics *valueMetrics) (int, int, error) {
	if metrics != nil && metrics.digitsSet {
		return metrics.totalDigits, metrics.fractionDigits, nil
	}
	if kind != runtime.VDecimal && kind != runtime.VInteger {
		return 0, 0, valueErrorf(valueErrInvalid, "digits facet not applicable")
	}
	total := 0
	fraction := 0
	switch kind {
	case runtime.VDecimal:
		var dec num.Dec
		if metrics != nil && metrics.decSet {
			dec = metrics.decVal
		} else {
			decVal, perr := num.ParseDec(canonical)
			if perr != nil {
				return 0, 0, valueErrorMsg(valueErrInvalid, "invalid decimal")
			}
			dec = decVal
		}
		total = len(dec.Coef)
		fraction = int(dec.Scale)
	case runtime.VInteger:
		var intVal num.Int
		if metrics != nil && metrics.intSet {
			intVal = metrics.intVal
		} else {
			val, perr := num.ParseInt(canonical)
			if perr != nil {
				return 0, 0, valueErrorMsg(valueErrInvalid, "invalid integer")
			}
			intVal = val
		}
		total = len(intVal.Digits)
		fraction = 0
	}
	if metrics != nil {
		metrics.totalDigits = total
		metrics.fractionDigits = fraction
		metrics.digitsSet = true
	}
	return total, fraction, nil
}

func shouldSkipRuntimeLengthFacet(kind runtime.ValidatorKind) bool {
	return kind == runtime.VQName || kind == runtime.VNotation
}

// deriveKeyFromCanonical computes the typed key from canonical bytes for enum checking.
// For string-like types, the key is the canonical bytes. For numeric types, we compute
// the binary key representation.
func (s *Session) deriveKeyFromCanonical(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	switch kind {
	case runtime.VString:
		key := stringKeyBytes(s.keyTmp[:0], 0, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VBoolean:
		switch {
		case bytes.Equal(canonical, []byte("true")):
			return runtime.VKBool, []byte{1}, nil
		case bytes.Equal(canonical, []byte("false")):
			return runtime.VKBool, []byte{0}, nil
		default:
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid boolean")
		}
	case runtime.VDecimal:
		decVal, perr := num.ParseDec(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		key := num.EncodeDecKey(s.keyTmp[:0], decVal)
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VInteger:
		intVal, perr := num.ParseInt(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		key := num.EncodeIntKey(s.keyTmp[:0], intVal)
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VFloat:
		v, class, perr := num.ParseFloat32(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid float")
		}
		key := float32KeyBytes(s.keyTmp[:0], v, class)
		s.keyTmp = key
		return runtime.VKFloat32, key, nil
	case runtime.VDouble:
		v, class, perr := num.ParseFloat64(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid double")
		}
		key := float64KeyBytes(s.keyTmp[:0], v, class)
		s.keyTmp = key
		return runtime.VKFloat64, key, nil
	case runtime.VAnyURI:
		key := stringKeyBytes(s.keyTmp[:0], 1, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VQName, runtime.VNotation:
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := qnameKeyBytes(s.keyTmp[:0], tag, canonical)
		if len(key) == 0 {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "invalid QName key")
		}
		s.keyTmp = key
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := types.ParseHexBinary(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := binaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VBase64Binary:
		decoded, err := types.ParseBase64Binary(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := binaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VDuration:
		dur, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := durationKeyBytes(s.keyTmp[:0], dur)
		s.keyTmp = key
		return runtime.VKDuration, key, nil
	case runtime.VDateTime, runtime.VDate, runtime.VTime, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		t, hasTZ, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		key := temporalKeyBytes(s.keyTmp[:0], temporalSubkind(kind), t, hasTZ)
		s.keyTmp = key
		return runtime.VKDateTime, key, nil
	default:
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", kind)
	}
}

func (s *Session) stringKind(meta runtime.ValidatorMeta) (runtime.StringKind, bool) {
	if int(meta.Index) >= len(s.rt.Validators.String) {
		return runtime.StringAny, false
	}
	return s.rt.Validators.String[meta.Index].Kind, true
}

func (s *Session) integerKind(meta runtime.ValidatorMeta) (runtime.IntegerKind, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Integer) {
		return runtime.IntegerAny, false
	}
	return s.rt.Validators.Integer[meta.Index].Kind, true
}

func (s *Session) listItemValidator(meta runtime.ValidatorMeta) (runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(s.rt.Validators.List) {
		return 0, false
	}
	return s.rt.Validators.List[meta.Index].Item, true
}

func (s *Session) unionMemberInfo(meta runtime.ValidatorMeta) ([]runtime.ValidatorID, []runtime.TypeID, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Union) {
		return nil, nil, false
	}
	union := s.rt.Validators.Union[meta.Index]
	end := union.MemberOff + union.MemberLen
	if int(end) > len(s.rt.Validators.UnionMembers) || int(end) > len(s.rt.Validators.UnionMemberTypes) {
		return nil, nil, false
	}
	return s.rt.Validators.UnionMembers[union.MemberOff:end], s.rt.Validators.UnionMemberTypes[union.MemberOff:end], true
}

func validateStringKind(kind runtime.StringKind, normalized []byte) error {
	switch kind {
	case runtime.StringToken:
		return value.ValidateToken(normalized)
	case runtime.StringLanguage:
		return value.ValidateLanguage(normalized)
	case runtime.StringName:
		return value.ValidateName(normalized)
	case runtime.StringNCName:
		return value.ValidateNCName(normalized)
	case runtime.StringID, runtime.StringIDREF, runtime.StringEntity:
		return value.ValidateNCName(normalized)
	case runtime.StringNMTOKEN:
		return value.ValidateNMTOKEN(normalized)
	default:
		return nil
	}
}

func validateIntegerKind(kind runtime.IntegerKind, v num.Int) error {
	switch kind {
	case runtime.IntegerAny:
		return nil
	case runtime.IntegerLong:
		return validateIntRange(v, minInt64, maxInt64, "long")
	case runtime.IntegerInt:
		return validateIntRange(v, minInt32, maxInt32, "int")
	case runtime.IntegerShort:
		return validateIntRange(v, minInt16, maxInt16, "short")
	case runtime.IntegerByte:
		return validateIntRange(v, minInt8, maxInt8, "byte")
	case runtime.IntegerNonNegative:
		if v.Sign < 0 {
			return fmt.Errorf("nonNegativeInteger must be >= 0")
		}
		return nil
	case runtime.IntegerPositive:
		if v.Sign <= 0 {
			return fmt.Errorf("positiveInteger must be >= 1")
		}
		return nil
	case runtime.IntegerNonPositive:
		if v.Sign > 0 {
			return fmt.Errorf("nonPositiveInteger must be <= 0")
		}
		return nil
	case runtime.IntegerNegative:
		if v.Sign >= 0 {
			return fmt.Errorf("negativeInteger must be <= -1")
		}
		return nil
	case runtime.IntegerUnsignedLong:
		if v.Sign < 0 {
			return fmt.Errorf("unsignedLong must be >= 0")
		}
		return validateIntRange(v, intZero, maxUint64, "unsignedLong")
	case runtime.IntegerUnsignedInt:
		if v.Sign < 0 {
			return fmt.Errorf("unsignedInt must be >= 0")
		}
		return validateIntRange(v, intZero, maxUint32, "unsignedInt")
	case runtime.IntegerUnsignedShort:
		if v.Sign < 0 {
			return fmt.Errorf("unsignedShort must be >= 0")
		}
		return validateIntRange(v, intZero, maxUint16, "unsignedShort")
	case runtime.IntegerUnsignedByte:
		if v.Sign < 0 {
			return fmt.Errorf("unsignedByte must be >= 0")
		}
		return validateIntRange(v, intZero, maxUint8, "unsignedByte")
	default:
		return nil
	}
}

func validateIntRange(v, minValue, maxValue num.Int, label string) error {
	if v.Compare(minValue) < 0 || v.Compare(maxValue) > 0 {
		return fmt.Errorf("%s out of range", label)
	}
	return nil
}

func (s *Session) decForCanonical(canonical []byte, metrics *valueMetrics) (num.Dec, error) {
	if metrics != nil && metrics.decSet {
		return metrics.decVal, nil
	}
	val, perr := num.ParseDec(canonical)
	if perr != nil {
		return num.Dec{}, valueErrorMsg(valueErrInvalid, "invalid decimal")
	}
	if metrics != nil {
		metrics.decVal = val
		metrics.decSet = true
	}
	return val, nil
}

func (s *Session) intForCanonical(canonical []byte, metrics *valueMetrics) (num.Int, error) {
	if metrics != nil && metrics.intSet {
		return metrics.intVal, nil
	}
	val, perr := num.ParseInt(canonical)
	if perr != nil {
		return num.Int{}, valueErrorMsg(valueErrInvalid, "invalid integer")
	}
	if metrics != nil {
		metrics.intVal = val
		metrics.intSet = true
	}
	return val, nil
}

func (s *Session) float32ForCanonical(canonical []byte, metrics *valueMetrics) (float32, num.FloatClass, error) {
	if metrics != nil && metrics.float32Set {
		return metrics.float32Val, metrics.float32Class, nil
	}
	val, class, perr := num.ParseFloat32(canonical)
	if perr != nil {
		return 0, num.FloatFinite, valueErrorMsg(valueErrInvalid, "invalid float")
	}
	if metrics != nil {
		metrics.float32Val = val
		metrics.float32Class = class
		metrics.float32Set = true
	}
	return val, class, nil
}

func (s *Session) float64ForCanonical(canonical []byte, metrics *valueMetrics) (float64, num.FloatClass, error) {
	if metrics != nil && metrics.float64Set {
		return metrics.float64Val, metrics.float64Class, nil
	}
	val, class, perr := num.ParseFloat64(canonical)
	if perr != nil {
		return 0, num.FloatFinite, valueErrorMsg(valueErrInvalid, "invalid double")
	}
	if metrics != nil {
		metrics.float64Val = val
		metrics.float64Class = class
		metrics.float64Set = true
	}
	return val, class, nil
}

func (s *Session) checkFloat32Range(op runtime.FacetOp, canonical, bound []byte, metrics *valueMetrics) error {
	val, valClass, err := s.float32ForCanonical(canonical, metrics)
	if err != nil {
		return err
	}
	boundVal, boundClass, perr := num.ParseFloat32(bound)
	if perr != nil {
		return valueErrorMsg(valueErrInvalid, "invalid float")
	}
	if boundClass == num.FloatNaN {
		if op == runtime.FMinInclusive || op == runtime.FMaxInclusive {
			if valClass == num.FloatNaN {
				return nil
			}
		}
		return rangeViolation(op)
	}
	if valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat32(val, valClass, boundVal, boundClass)
	switch op {
	case runtime.FMinInclusive:
		if cmp < 0 {
			return valueErrorf(valueErrFacet, "minInclusive violation")
		}
	case runtime.FMaxInclusive:
		if cmp > 0 {
			return valueErrorf(valueErrFacet, "maxInclusive violation")
		}
	case runtime.FMinExclusive:
		if cmp <= 0 {
			return valueErrorf(valueErrFacet, "minExclusive violation")
		}
	case runtime.FMaxExclusive:
		if cmp >= 0 {
			return valueErrorf(valueErrFacet, "maxExclusive violation")
		}
	}
	return nil
}

func (s *Session) checkFloat64Range(op runtime.FacetOp, canonical, bound []byte, metrics *valueMetrics) error {
	val, valClass, err := s.float64ForCanonical(canonical, metrics)
	if err != nil {
		return err
	}
	boundVal, boundClass, perr := num.ParseFloat64(bound)
	if perr != nil {
		return valueErrorMsg(valueErrInvalid, "invalid double")
	}
	if boundClass == num.FloatNaN {
		if op == runtime.FMinInclusive || op == runtime.FMaxInclusive {
			if valClass == num.FloatNaN {
				return nil
			}
		}
		return rangeViolation(op)
	}
	if valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat64(val, valClass, boundVal, boundClass)
	switch op {
	case runtime.FMinInclusive:
		if cmp < 0 {
			return valueErrorf(valueErrFacet, "minInclusive violation")
		}
	case runtime.FMaxInclusive:
		if cmp > 0 {
			return valueErrorf(valueErrFacet, "maxInclusive violation")
		}
	case runtime.FMinExclusive:
		if cmp <= 0 {
			return valueErrorf(valueErrFacet, "minExclusive violation")
		}
	case runtime.FMaxExclusive:
		if cmp >= 0 {
			return valueErrorf(valueErrFacet, "maxExclusive violation")
		}
	}
	return nil
}

func rangeViolation(op runtime.FacetOp) error {
	switch op {
	case runtime.FMinInclusive:
		return valueErrorf(valueErrFacet, "minInclusive violation")
	case runtime.FMaxInclusive:
		return valueErrorf(valueErrFacet, "maxInclusive violation")
	case runtime.FMinExclusive:
		return valueErrorf(valueErrFacet, "minExclusive violation")
	case runtime.FMaxExclusive:
		return valueErrorf(valueErrFacet, "maxExclusive violation")
	default:
		return valueErrorf(valueErrFacet, "range violation")
	}
}

func stringKeyBytes(dst []byte, tag byte, value []byte) []byte {
	dst = append(dst[:0], tag)
	dst = append(dst, value...)
	return dst
}

func binaryKeyBytes(dst []byte, tag byte, data []byte) []byte {
	dst = append(dst[:0], tag)
	dst = append(dst, data...)
	return dst
}

func qnameKeyBytes(dst []byte, tag byte, canonical []byte) []byte {
	sep := bytes.IndexByte(canonical, 0)
	if sep < 0 {
		return nil
	}
	ns := canonical[:sep]
	local := canonical[sep+1:]
	dst = append(dst[:0], tag)
	dst = appendUvarint(dst, uint64(len(ns)))
	dst = append(dst, ns...)
	dst = appendUvarint(dst, uint64(len(local)))
	dst = append(dst, local...)
	return dst
}

const (
	canonicalNaN32 = 0x7fc00000
	canonicalNaN64 = 0x7ff8000000000000
)

func float32KeyBytes(dst []byte, value float32, class num.FloatClass) []byte {
	var bits uint32
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN32
	default:
		if value == 0 {
			bits = 0
		} else {
			bits = math.Float32bits(value)
		}
	}
	dst = ensureLen(dst[:0], 4)
	binary.BigEndian.PutUint32(dst, bits)
	return dst
}

func float64KeyBytes(dst []byte, value float64, class num.FloatClass) []byte {
	var bits uint64
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN64
	default:
		if value == 0 {
			bits = 0
		} else {
			bits = math.Float64bits(value)
		}
	}
	dst = ensureLen(dst[:0], 8)
	binary.BigEndian.PutUint64(dst, bits)
	return dst
}

func temporalSubkind(kind runtime.ValidatorKind) byte {
	switch kind {
	case runtime.VDateTime:
		return 0
	case runtime.VDate:
		return 1
	case runtime.VTime:
		return 2
	case runtime.VGYearMonth:
		return 3
	case runtime.VGYear:
		return 4
	case runtime.VGMonthDay:
		return 5
	case runtime.VGDay:
		return 6
	case runtime.VGMonth:
		return 7
	default:
		return 0
	}
}

func parseTemporalForKind(kind runtime.ValidatorKind, lexical []byte) (time.Time, bool, error) {
	switch kind {
	case runtime.VDateTime:
		t, err := value.ParseDateTime(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VDate:
		t, err := value.ParseDate(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VTime:
		t, err := value.ParseTime(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VGYearMonth:
		t, err := value.ParseGYearMonth(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VGYear:
		t, err := value.ParseGYear(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VGMonthDay:
		t, err := value.ParseGMonthDay(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VGDay:
		t, err := value.ParseGDay(lexical)
		return t, value.HasTimezone(lexical), err
	case runtime.VGMonth:
		t, err := value.ParseGMonth(lexical)
		return t, value.HasTimezone(lexical), err
	default:
		return time.Time{}, false, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
}

func temporalKeyBytes(dst []byte, subkind byte, t time.Time, hasTZ bool) []byte {
	if hasTZ {
		utc := t.UTC()
		dst = ensureLen(dst[:0], 14)
		dst[0] = subkind
		dst[1] = 1
		binary.BigEndian.PutUint64(dst[2:], uint64(utc.Unix()))
		binary.BigEndian.PutUint32(dst[10:], uint32(utc.Nanosecond()))
		return dst
	}
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	dst = ensureLen(dst[:0], 20)
	dst[0] = subkind
	dst[1] = 0
	binary.BigEndian.PutUint32(dst[2:], uint32(int32(year)))
	binary.BigEndian.PutUint16(dst[6:], uint16(month))
	binary.BigEndian.PutUint16(dst[8:], uint16(day))
	binary.BigEndian.PutUint16(dst[10:], uint16(hour))
	binary.BigEndian.PutUint16(dst[12:], uint16(min))
	binary.BigEndian.PutUint16(dst[14:], uint16(sec))
	binary.BigEndian.PutUint32(dst[16:], uint32(t.Nanosecond()))
	return dst
}

func durationKeyBytes(dst []byte, dur types.XSDDuration) []byte {
	monthsTotal := int64(dur.Years)*12 + int64(dur.Months)
	months, _ := num.ParseInt([]byte(strconv.FormatInt(monthsTotal, 10)))
	secondsTotal := float64(dur.Days)*86400 + float64(dur.Hours)*3600 + float64(dur.Minutes)*60 + dur.Seconds
	if secondsTotal < 0 {
		secondsTotal = -secondsTotal
	}
	secStr := strconv.FormatFloat(secondsTotal, 'f', -1, 64)
	seconds, _ := num.ParseDec([]byte(secStr))
	sign := byte(1)
	if dur.Negative {
		sign = 2
	}
	if monthsTotal == 0 && seconds.Sign == 0 {
		sign = 0
	}
	dst = append(dst[:0], sign)
	dst = num.EncodeIntKey(dst, months)
	dst = num.EncodeDecKey(dst, seconds)
	return dst
}

func appendUvarint(dst []byte, v uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	return append(dst, buf[:n]...)
}

func ensureLen(dst []byte, n int) []byte {
	if cap(dst) < n {
		return make([]byte, n)
	}
	return dst[:n]
}

func forEachListItem(normalized []byte, spaceOnly bool, fn func([]byte) error) (int, error) {
	if len(normalized) == 0 {
		return 0, nil
	}
	count := 0
	i := 0
	if spaceOnly {
		for i < len(normalized) {
			for i < len(normalized) && normalized[i] == ' ' {
				i++
			}
			if i >= len(normalized) {
				break
			}
			j := bytes.IndexByte(normalized[i:], ' ')
			if j < 0 {
				j = len(normalized)
			} else {
				j += i
			}
			if fn != nil {
				if err := fn(normalized[i:j]); err != nil {
					return count, err
				}
			}
			count++
			i = j
		}
		return count, nil
	}
	for i < len(normalized) {
		for i < len(normalized) && isXMLWhitespace(normalized[i]) {
			i++
		}
		if i >= len(normalized) {
			break
		}
		start := i
		for i < len(normalized) && !isXMLWhitespace(normalized[i]) {
			i++
		}
		if start < i {
			if fn != nil {
				if err := fn(normalized[start:i]); err != nil {
					return count, err
				}
			}
			count++
		}
	}
	return count, nil
}

func listItemCount(normalized []byte, spaceOnly bool) int {
	count, _ := forEachListItem(normalized, spaceOnly, nil)
	return count
}

func isXMLWhitespace(b byte) bool {
	if b > ' ' {
		return false
	}
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func bytesIndexByte(b []byte, needle byte) int {
	for i := range b {
		if b[i] == needle {
			return i
		}
	}
	return -1
}

func trimLeftZeros(b []byte) []byte {
	for len(b) > 0 && b[0] == '0' {
		b = b[1:]
	}
	return b
}

func (s *Session) trackIDs(kind runtime.StringKind, canonical []byte) error {
	switch kind {
	case runtime.StringID:
		return s.recordID(canonical)
	case runtime.StringIDREF:
		s.recordIDRef(canonical)
	case runtime.StringEntity:
		// ENTITY validation handled elsewhere
	}
	return nil
}

func (s *Session) trackDefaultValue(id runtime.ValidatorID, canonical []byte) error {
	if s == nil || s.rt == nil || id == 0 {
		return nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return fmt.Errorf("validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return fmt.Errorf("string validator out of range")
		}
		return s.trackIDs(kind, canonical)
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return fmt.Errorf("list validator out of range")
		}
		if _, err := forEachListItem(canonical, meta.WhiteSpace == runtime.WS_Collapse, func(itemValue []byte) error {
			if err := s.trackDefaultValue(item, itemValue); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	case runtime.VUnion:
		members, _, ok := s.unionMemberInfo(meta)
		if !ok || len(members) == 0 {
			return fmt.Errorf("union validator out of range")
		}
		for _, member := range members {
			if _, err := s.validateValueInternalNoTrack(member, canonical, nil, true); err == nil {
				return s.trackDefaultValue(member, canonical)
			}
		}
	}
	return nil
}

func (s *Session) keyForCanonicalValue(id runtime.ValidatorID, canonical []byte) (runtime.ValueKind, []byte, error) {
	if s == nil || s.rt == nil || id == 0 {
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "validator missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "list validator out of range")
		}
		var keyBytes []byte
		count := 0
		spaceOnly := meta.WhiteSpace == runtime.WS_Collapse
		if _, err := forEachListItem(canonical, spaceOnly, func(itemValue []byte) error {
			kind, key, err := s.keyForCanonicalValue(item, itemValue)
			if err != nil {
				return err
			}
			keyBytes = runtime.AppendListKey(keyBytes, kind, key)
			count++
			return nil
		}); err != nil {
			return runtime.VKInvalid, nil, err
		}
		listKey := appendUvarint(s.keyTmp[:0], uint64(count))
		listKey = append(listKey, keyBytes...)
		s.keyTmp = listKey
		return runtime.VKList, listKey, nil
	case runtime.VUnion:
		members, _, ok := s.unionMemberInfo(meta)
		if !ok || len(members) == 0 {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "union validator out of range")
		}
		for _, member := range members {
			kind, key, err := s.keyForCanonicalValue(member, canonical)
			if err == nil {
				return kind, key, nil
			}
		}
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "union value does not match any member type")
	default:
		return s.deriveKeyFromCanonical(meta.Kind, canonical)
	}
}

func (s *Session) recordID(valueBytes []byte) error {
	if s == nil {
		return nil
	}
	if s.idTable == nil {
		s.idTable = make(map[string]struct{}, 32)
	}
	key := string(valueBytes)
	if _, ok := s.idTable[key]; ok {
		return newValidationError(xsderrors.ErrDuplicateID, "duplicate ID value")
	}
	s.idTable[key] = struct{}{}
	return nil
}

func (s *Session) recordIDRef(valueBytes []byte) {
	if s == nil {
		return
	}
	s.idRefs = append(s.idRefs, string(valueBytes))
}

func (s *Session) validateIDRefs() []error {
	if s == nil {
		return nil
	}
	if len(s.idRefs) == 0 {
		return nil
	}
	var errs []error
	for _, ref := range s.idRefs {
		if _, ok := s.idTable[ref]; !ok {
			errs = append(errs, newValidationError(xsderrors.ErrIDRefNotFound, "IDREF value not found"))
		}
	}
	return errs
}

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

func encodeBase64(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}
