package value

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
)

// BinaryValue is the value-space projection for binary primitive values.
type BinaryValue lexicalValue

// ParseBinaryValue parses normalized as an XML Schema binary primitive value.
func ParseBinaryValue(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (BinaryValue, error) {
	switch kind {
	case PrimitiveHexBinary:
		return parseHexBinaryValue(normalized, needs)
	case PrimitiveBase64Binary:
		return parseBase64BinaryValue(normalized, needs)
	case PrimitiveString, PrimitiveBoolean, PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble,
		PrimitiveDuration, PrimitiveDateTime, PrimitiveTime, PrimitiveDate,
		PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth,
		PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return BinaryValue{}, errors.New(msgInvalidBinaryPrimitive)
	default:
	}
	return BinaryValue{}, errors.New(msgInvalidBinaryPrimitive)
}

// ValidateHexBinaryLexical validates raw as an XML Schema hexBinary lexical
// value.
func ValidateHexBinaryLexical[T byteText](raw T) error {
	if len(raw)%2 != 0 {
		return errors.New(msgInvalidHexBinary)
	}
	for i := range len(raw) {
		if !isHexDigit(raw[i]) {
			return errors.New(msgInvalidHexBinary)
		}
	}
	return nil
}

func parseHexBinaryValue(normalized string, needs PrimitiveValueNeed) (BinaryValue, error) {
	length, err := hexBinaryLength(normalized)
	if err != nil {
		return BinaryValue{}, err
	}
	canonical := normalized
	if needs.Has(PrimitiveNeedCanonical) {
		canonical = strings.ToUpper(normalized)
	}
	return BinaryValue{Canonical: canonical, Length: length}, nil
}

func hexBinaryLength(normalized string) (uint32, error) {
	if err := ValidateHexBinaryLexical(normalized); err != nil {
		return 0, err
	}
	return checkedUint32(len(normalized)/2, msgHexBinaryLengthLimit)
}

// ValidateBase64BinaryLexical validates raw as an XML Schema base64Binary
// lexical value.
func ValidateBase64BinaryLexical[T byteText](raw T) error {
	_, err := scanBase64BinaryLexical(raw)
	return err
}

func parseBase64BinaryValue(normalized string, needs PrimitiveValueNeed) (BinaryValue, error) {
	scan, err := scanBase64BinaryLexical(normalized)
	if err != nil {
		return BinaryValue{}, err
	}
	length, err := checkedUint32(scan.cleanLen/4*3-scan.pads, msgBase64BinaryLengthLimit)
	if err != nil {
		return BinaryValue{}, err
	}
	if needs.Has(PrimitiveNeedCanonical) {
		return BinaryValue{
			Canonical:      removeXMLWhitespace(normalized),
			Length:         length,
			canonicalReady: true,
		}, nil
	}
	return BinaryValue{Canonical: normalized, Length: length}, nil
}

func decodeBase64Binary(normalized string) ([]byte, error) {
	if _, err := scanBase64BinaryLexical(normalized); err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(removeXMLWhitespace(normalized))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

type base64BinaryScan struct {
	cleanLen int
	pads     int
}

//nolint:gocognit // One scan owns alphabet admission, padding order, and final pad bits.
func scanBase64BinaryLexical[T byteText](raw T) (base64BinaryScan, error) {
	var scan base64BinaryScan
	var lastValue byte
	for i := range len(raw) {
		b := raw[i]
		if lex.IsXMLWhitespaceByte(b) {
			continue
		}
		if b == '=' {
			scan.pads++
			scan.cleanLen++
			continue
		}
		if scan.pads > 0 {
			return base64BinaryScan{}, errors.New(msgInvalidBase64Binary)
		}
		v, ok := base64Value(b)
		if !ok {
			return base64BinaryScan{}, errors.New(msgInvalidBase64Binary)
		}
		scan.cleanLen++
		lastValue = v
	}
	if scan.cleanLen%4 != 0 || scan.pads > 2 {
		return base64BinaryScan{}, errors.New(msgInvalidBase64Binary)
	}
	if scan.pads == 1 && lastValue&0x03 != 0 || scan.pads == 2 && lastValue&0x0f != 0 {
		return base64BinaryScan{}, errors.New(msgInvalidBase64Binary)
	}
	return scan, nil
}

func isHexDigit(b byte) bool {
	return '0' <= b && b <= '9' || 'a' <= b && b <= 'f' || 'A' <= b && b <= 'F'
}

func base64Value(b byte) (byte, bool) {
	switch {
	case 'A' <= b && b <= 'Z':
		return b - 'A', true
	case 'a' <= b && b <= 'z':
		return b - 'a' + 26, true
	case '0' <= b && b <= '9':
		return b - '0' + 52, true
	case b == '+':
		return 62, true
	case b == '/':
		return 63, true
	default:
		return 0, false
	}
}

func removeXMLWhitespace(s string) string {
	i := -1
	for pos := range len(s) {
		if lex.IsXMLWhitespaceByte(s[pos]) {
			i = pos
			break
		}
	}
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		if !lex.IsXMLWhitespaceByte(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
