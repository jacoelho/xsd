package validator

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode/utf8"

	xsderrors "github.com/jacoelho/xsd/errors"
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

const (
	minInt8  = -1 << 7
	maxInt8  = 1<<7 - 1
	minInt16 = -1 << 15
	maxInt16 = 1<<15 - 1
	minInt32 = -1 << 31
	maxInt32 = 1<<31 - 1
	minInt64 = -1 << 63
	maxInt64 = 1<<63 - 1
)

var (
	bigMinInt8   = big.NewInt(minInt8)
	bigMaxInt8   = big.NewInt(maxInt8)
	bigMinInt16  = big.NewInt(minInt16)
	bigMaxInt16  = big.NewInt(maxInt16)
	bigMinInt32  = big.NewInt(minInt32)
	bigMaxInt32  = big.NewInt(maxInt32)
	bigMinInt64  = big.NewInt(minInt64)
	bigMaxInt64  = big.NewInt(maxInt64)
	bigMaxUint8  = new(big.Int).SetUint64(1<<8 - 1)
	bigMaxUint16 = new(big.Int).SetUint64(1<<16 - 1)
	bigMaxUint32 = new(big.Int).SetUint64(1<<32 - 1)
	bigMaxUint64 = new(big.Int).SetUint64(^uint64(0))
)

type valueMetrics struct {
	comp           types.ComparableValue
	keyBytes       []byte
	fractionDigits int
	totalDigits    int
	listCount      int
	length         int
	keyKind        runtime.ValueKind
	lengthSet      bool
	digitsSet      bool
	listSet        bool
	compSet        bool
	keySet         bool
	patternChecked bool
	enumChecked    bool
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
	// only force key computation for unions with enums (they check enum membership inline)
	// for atomic types, keys are computed lazily in applyFacets if needed
	needKey := opts.needKey || opts.storeValue || (meta.Kind == runtime.VUnion && meta.Flags&runtime.ValidatorHasEnum != 0)
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
			s.setKey(metrics, runtime.VKString, canon, opts.storeValue)
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
			s.setKey(metrics, runtime.VKBool, canon, opts.storeValue)
		}
		return canon, nil
	case runtime.VDecimal:
		if _, err := value.ParseDecimal(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonRaw := value.CanonicalDecimalBytes(normalized, nil)
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key, err := value.CanonicalDecimalKeyFromCanonical(canonRaw, s.keyTmp[:0])
			if err != nil {
				return nil, valueErrorMsg(valueErrInvalid, err.Error())
			}
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDecimal, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VInteger:
		kind, ok := s.integerKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "integer validator out of range")
		}
		v, err := value.ParseInteger(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if err := validateIntegerKind(kind, v); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonRaw := []byte(v.String())
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key, err := value.CanonicalIntegerKeyFromCanonical(canonRaw, s.keyTmp[:0])
			if err != nil {
				return nil, valueErrorMsg(valueErrInvalid, err.Error())
			}
			s.keyTmp = key
			s.setKey(metrics, runtime.VKInteger, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VFloat:
		v, err := value.ParseFloat(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonRaw := []byte(value.CanonicalFloat(float64(v), 32))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := value.CanonicalFloat32Key(v, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKFloat32, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VDouble:
		v, err := value.ParseDouble(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonRaw := []byte(value.CanonicalFloat(v, 64))
		canon := s.maybeStore(canonRaw, opts.storeValue)
		if needKey {
			key := value.CanonicalFloat64Key(v, s.keyTmp[:0])
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
			key := types.CanonicalDurationKey(dur, s.keyTmp[:0])
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
			key := value.CanonicalTemporalKey(t, "dateTime", hasTZ, s.keyTmp[:0])
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
			key := value.CanonicalTemporalKey(t, "time", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKTime, key, opts.storeValue)
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
			key := value.CanonicalTemporalKey(t, "date", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKDate, key, opts.storeValue)
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
			key := value.CanonicalTemporalKey(t, "gYearMonth", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKGYearMonth, key, opts.storeValue)
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
			key := value.CanonicalTemporalKey(t, "gYear", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKGYear, key, opts.storeValue)
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
			key := value.CanonicalTemporalKey(t, "gMonthDay", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKGMonthDay, key, opts.storeValue)
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
			key := value.CanonicalTemporalKey(t, "gDay", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKGDay, key, opts.storeValue)
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
			key := value.CanonicalTemporalKey(t, "gMonth", hasTZ, s.keyTmp[:0])
			s.keyTmp = key
			s.setKey(metrics, runtime.VKGMonth, key, opts.storeValue)
		}
		return canon, nil
	case runtime.VAnyURI:
		if err := value.ValidateAnyURI(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canon := s.maybeStore(normalized, opts.storeValue)
		if needKey {
			s.setKey(metrics, runtime.VKAnyURI, canon, opts.storeValue)
		}
		return canon, nil
	case runtime.VQName, runtime.VNotation:
		canon, err := value.CanonicalQName(normalized, resolver, nil)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canonStored := s.maybeStore(canon, opts.storeValue)
		if needKey {
			s.setKey(metrics, runtime.VKQName, canonStored, opts.storeValue)
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
			s.setKey(metrics, runtime.VKHexBinary, canon, opts.storeValue)
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
			s.setKey(metrics, runtime.VKBase64Binary, canon, opts.storeValue)
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
			s.setKey(metrics, runtime.VKList, keyTmp, opts.storeValue)
		}
		return canon, nil
	case runtime.VUnion:
		memberValidators, ok := s.unionMemberValidators(meta)
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
		for _, member := range memberValidators {
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
		if _, err := value.ParseDecimal(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VInteger:
		kind, ok := s.integerKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "integer validator out of range")
		}
		v, err := value.ParseInteger(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if err := validateIntegerKind(kind, v); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VFloat:
		if err := value.ValidateFloatLexical(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return s.maybeStore(normalized, opts.storeValue), nil
	case runtime.VDouble:
		if err := value.ValidateDoubleLexical(normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
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
				if !runtime.EnumContains(&s.rt.Enums, s.rt.Values, enumID, kind, key) {
					return valueErrorf(valueErrFacet, "enumeration violation")
				}
			} else if !runtime.EnumContains(&s.rt.Enums, s.rt.Values, enumID, metrics.keyKind, metrics.keyBytes) {
				return valueErrorf(valueErrFacet, "enumeration violation")
			}
		case runtime.FMinInclusive, runtime.FMaxInclusive, runtime.FMinExclusive, runtime.FMaxExclusive:
			ref := runtime.ValueRef{Off: instr.Arg0, Len: instr.Arg1, Present: true}
			bound := valueBytes(s.rt.Values, ref)
			if bound == nil {
				return valueErrorf(valueErrInvalid, "range facet bound out of range")
			}
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
		if !runtime.EnumContains(&s.rt.Enums, s.rt.Values, enumID, keyKind, keyBytes) {
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
	if metrics != nil && metrics.compSet {
		boundComp, err := comparableForKind(kind, bound)
		if err != nil {
			return 0, err
		}
		cmp, err := metrics.comp.Compare(boundComp)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	}
	comp, err := comparableForKind(kind, canonical)
	if err != nil {
		return 0, err
	}
	if metrics != nil {
		metrics.comp = comp
		metrics.compSet = true
	}
	boundComp, err := comparableForKind(kind, bound)
	if err != nil {
		return 0, err
	}
	cmp, err := comp.Compare(boundComp)
	if err != nil {
		return 0, valueErrorMsg(valueErrFacet, err.Error())
	}
	return cmp, nil
}

func comparableForKind(kind runtime.ValidatorKind, lexical []byte) (types.ComparableValue, error) {
	switch kind {
	case runtime.VDecimal:
		val, err := value.ParseDecimal(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableBigRat{Value: val}, nil
	case runtime.VInteger:
		val, err := value.ParseInteger(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableBigInt{Value: val}, nil
	case runtime.VFloat:
		val, err := value.ParseFloat(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if math.IsNaN(float64(val)) {
			return nil, valueErrorf(valueErrInvalid, "NaN not comparable")
		}
		return types.ComparableFloat32{Value: val}, nil
	case runtime.VDouble:
		val, err := value.ParseDouble(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if math.IsNaN(val) {
			return nil, valueErrorf(valueErrInvalid, "NaN not comparable")
		}
		return types.ComparableFloat64{Value: val}, nil
	case runtime.VDuration:
		val, err := types.ParseXSDDuration(string(lexical))
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableXSDDuration{Value: val}, nil
	case runtime.VDateTime:
		t, err := value.ParseDateTime(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VTime:
		t, err := value.ParseTime(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VDate:
		t, err := value.ParseDate(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VGYearMonth:
		t, err := value.ParseGYearMonth(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VGYear:
		t, err := value.ParseGYear(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VGMonthDay:
		t, err := value.ParseGMonthDay(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VGDay:
		t, err := value.ParseGDay(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	case runtime.VGMonth:
		t, err := value.ParseGMonth(lexical)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		return types.ComparableTime{Value: t, HasTimezone: value.HasTimezone(lexical)}, nil
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported comparable type %d", kind)
	}
}

func digitCounts(kind runtime.ValidatorKind, canonical []byte, metrics *valueMetrics) (int, int, error) {
	if metrics != nil && metrics.digitsSet {
		return metrics.totalDigits, metrics.fractionDigits, nil
	}
	if kind != runtime.VDecimal && kind != runtime.VInteger {
		return 0, 0, valueErrorf(valueErrInvalid, "digits facet not applicable")
	}
	b := canonical
	if len(b) > 0 && (b[0] == '+' || b[0] == '-') {
		b = b[1:]
	}
	total := 0
	fraction := 0
	if idx := bytesIndexByte(b, '.'); idx >= 0 {
		intPart := trimLeftZeros(b[:idx])
		fraction = len(b) - idx - 1
		total = len(intPart) + fraction
	} else {
		intPart := trimLeftZeros(b)
		total = len(intPart)
		fraction = 0
	}
	if total == 0 {
		total = 1
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
		return runtime.VKString, canonical, nil
	case runtime.VBoolean:
		return runtime.VKBool, canonical, nil
	case runtime.VDecimal:
		key, err := value.CanonicalDecimalKeyFromCanonical(canonical, s.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VInteger:
		key, err := value.CanonicalIntegerKeyFromCanonical(canonical, s.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		s.keyTmp = key
		return runtime.VKInteger, key, nil
	case runtime.VFloat:
		v, err := value.ParseFloat(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := value.CanonicalFloat32Key(v, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKFloat32, key, nil
	case runtime.VDouble:
		v, err := value.ParseDouble(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := value.CanonicalFloat64Key(v, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKFloat64, key, nil
	case runtime.VAnyURI:
		return runtime.VKAnyURI, canonical, nil
	case runtime.VQName, runtime.VNotation:
		return runtime.VKQName, canonical, nil
	case runtime.VHexBinary:
		return runtime.VKHexBinary, canonical, nil
	case runtime.VBase64Binary:
		return runtime.VKBase64Binary, canonical, nil
	case runtime.VDuration:
		dur, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := types.CanonicalDurationKey(dur, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKDuration, key, nil
	case runtime.VDateTime, runtime.VDate, runtime.VTime, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		// for temporal types with enums, the canonical bytes are typically the key
		// (timezone presence is already encoded in canonical form)
		vkind, ok := runtime.ValueKindForValidatorKind(kind)
		if !ok {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "unknown temporal kind %d", kind)
		}
		return vkind, canonical, nil
	default:
		return runtime.VKString, canonical, nil
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

func (s *Session) unionMemberValidators(meta runtime.ValidatorMeta) ([]runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Union) {
		return nil, false
	}
	union := s.rt.Validators.Union[meta.Index]
	if int(union.MemberOff+union.MemberLen) > len(s.rt.Validators.UnionMembers) {
		return nil, false
	}
	return s.rt.Validators.UnionMembers[union.MemberOff : union.MemberOff+union.MemberLen], true
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

func validateIntegerKind(kind runtime.IntegerKind, v *big.Int) error {
	if v == nil {
		return fmt.Errorf("invalid integer")
	}
	switch kind {
	case runtime.IntegerAny:
		return nil
	case runtime.IntegerLong:
		return validateBoundedInt(v, bigMinInt64, bigMaxInt64, "long")
	case runtime.IntegerInt:
		return validateBoundedInt(v, bigMinInt32, bigMaxInt32, "int")
	case runtime.IntegerShort:
		return validateBoundedInt(v, bigMinInt16, bigMaxInt16, "short")
	case runtime.IntegerByte:
		return validateBoundedInt(v, bigMinInt8, bigMaxInt8, "byte")
	case runtime.IntegerNonNegative:
		if v.Sign() < 0 {
			return fmt.Errorf("nonNegativeInteger must be >= 0")
		}
		return nil
	case runtime.IntegerPositive:
		if v.Sign() <= 0 {
			return fmt.Errorf("positiveInteger must be >= 1")
		}
		return nil
	case runtime.IntegerNonPositive:
		if v.Sign() > 0 {
			return fmt.Errorf("nonPositiveInteger must be <= 0")
		}
		return nil
	case runtime.IntegerNegative:
		if v.Sign() >= 0 {
			return fmt.Errorf("negativeInteger must be <= -1")
		}
		return nil
	case runtime.IntegerUnsignedLong:
		if v.Sign() < 0 {
			return fmt.Errorf("unsignedLong must be >= 0")
		}
		if v.Cmp(bigMaxUint64) > 0 {
			return fmt.Errorf("unsignedLong out of range")
		}
		return nil
	case runtime.IntegerUnsignedInt:
		if v.Sign() < 0 {
			return fmt.Errorf("unsignedInt must be >= 0")
		}
		if v.Cmp(bigMaxUint32) > 0 {
			return fmt.Errorf("unsignedInt out of range")
		}
		return nil
	case runtime.IntegerUnsignedShort:
		if v.Sign() < 0 {
			return fmt.Errorf("unsignedShort must be >= 0")
		}
		if v.Cmp(bigMaxUint16) > 0 {
			return fmt.Errorf("unsignedShort out of range")
		}
		return nil
	case runtime.IntegerUnsignedByte:
		if v.Sign() < 0 {
			return fmt.Errorf("unsignedByte must be >= 0")
		}
		if v.Cmp(bigMaxUint8) > 0 {
			return fmt.Errorf("unsignedByte out of range")
		}
		return nil
	default:
		return nil
	}
}

func validateBoundedInt(v, minValue, maxValue *big.Int, label string) error {
	if v.Cmp(minValue) < 0 || v.Cmp(maxValue) > 0 {
		return fmt.Errorf("%s out of range", label)
	}
	return nil
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
		members, ok := s.unionMemberValidators(meta)
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
	case runtime.VString:
		return runtime.VKString, canonical, nil
	case runtime.VBoolean:
		v, err := value.ParseBoolean(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if v {
			return runtime.VKBool, []byte("true"), nil
		}
		return runtime.VKBool, []byte("false"), nil
	case runtime.VDecimal:
		key, err := value.CanonicalDecimalKeyFromCanonical(canonical, s.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VInteger:
		key, err := value.CanonicalIntegerKeyFromCanonical(canonical, s.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		s.keyTmp = key
		return runtime.VKInteger, key, nil
	case runtime.VFloat:
		v, err := value.ParseFloat(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := value.CanonicalFloat32Key(v, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKFloat32, key, nil
	case runtime.VDouble:
		v, err := value.ParseDouble(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := value.CanonicalFloat64Key(v, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKFloat64, key, nil
	case runtime.VDuration:
		dur, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := types.CanonicalDurationKey(dur, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKDuration, key, nil
	case runtime.VDateTime:
		t, err := value.ParseDateTime(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "dateTime", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKDateTime, key, nil
	case runtime.VTime:
		t, err := value.ParseTime(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "time", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKTime, key, nil
	case runtime.VDate:
		t, err := value.ParseDate(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "date", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKDate, key, nil
	case runtime.VGYearMonth:
		t, err := value.ParseGYearMonth(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "gYearMonth", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKGYearMonth, key, nil
	case runtime.VGYear:
		t, err := value.ParseGYear(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "gYear", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKGYear, key, nil
	case runtime.VGMonthDay:
		t, err := value.ParseGMonthDay(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "gMonthDay", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKGMonthDay, key, nil
	case runtime.VGDay:
		t, err := value.ParseGDay(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "gDay", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKGDay, key, nil
	case runtime.VGMonth:
		t, err := value.ParseGMonth(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		hasTZ := value.HasTimezone(canonical)
		key := value.CanonicalTemporalKey(t, "gMonth", hasTZ, s.keyTmp[:0])
		s.keyTmp = key
		return runtime.VKGMonth, key, nil
	case runtime.VAnyURI:
		return runtime.VKAnyURI, canonical, nil
	case runtime.VQName, runtime.VNotation:
		return runtime.VKQName, canonical, nil
	case runtime.VHexBinary:
		return runtime.VKHexBinary, canonical, nil
	case runtime.VBase64Binary:
		return runtime.VKBase64Binary, canonical, nil
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "list validator out of range")
		}
		var keyBytes []byte
		spaceOnly := meta.WhiteSpace == runtime.WS_Collapse
		if _, err := forEachListItem(canonical, spaceOnly, func(itemValue []byte) error {
			kind, key, err := s.keyForCanonicalValue(item, itemValue)
			if err != nil {
				return err
			}
			keyBytes = runtime.AppendListKey(keyBytes, kind, key)
			return nil
		}); err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKList, keyBytes, nil
	case runtime.VUnion:
		members, ok := s.unionMemberValidators(meta)
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
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
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
