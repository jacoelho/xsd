package value

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
)

func ValidateXSDAnyType(_ string) error { return nil }

func ValidateXSDAnySimpleType(_ string) error { return nil }

func ValidateXSDString(_ string) error { return nil }

func ValidateXSDBoolean(lexical string) error {
	_, err := ParseBoolean([]byte(lexical))
	return err
}

func ValidateXSDDecimal(lexical string) error {
	_, err := ParseDecimal([]byte(lexical))
	return err
}

func ValidateXSDFloat(lexical string) error {
	if perr := num.ValidateFloatLexical([]byte(lexical)); perr != nil {
		return fmt.Errorf("invalid float: %s", lexical)
	}
	return nil
}

func ValidateXSDDouble(lexical string) error {
	if perr := num.ValidateFloatLexical([]byte(lexical)); perr != nil {
		return fmt.Errorf("invalid double: %s", lexical)
	}
	return nil
}

func ValidateXSDInteger(lexical string) error {
	_, err := ParseInteger([]byte(lexical))
	return err
}

func validateSignedInt(lexical, label string) (int64, error) {
	if err := ValidateXSDInteger(lexical); err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", label, lexical)
	}
	return n, nil
}

func ValidateXSDLong(lexical string) error {
	_, err := validateSignedInt(lexical, "long")
	return err
}

func ValidateXSDInt(lexical string) error {
	n, err := validateSignedInt(lexical, "int")
	if err != nil {
		return err
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return fmt.Errorf("int out of range: %s", lexical)
	}
	return nil
}

func ValidateXSDShort(lexical string) error {
	n, err := validateSignedInt(lexical, "short")
	if err != nil {
		return err
	}
	if n < math.MinInt16 || n > math.MaxInt16 {
		return fmt.Errorf("short out of range: %s", lexical)
	}
	return nil
}

func ValidateXSDByte(lexical string) error {
	n, err := validateSignedInt(lexical, "byte")
	if err != nil {
		return err
	}
	if n < math.MinInt8 || n > math.MaxInt8 {
		return fmt.Errorf("byte out of range: %s", lexical)
	}
	return nil
}

func ValidateXSDNonNegativeInteger(lexical string) error {
	if err := ValidateXSDInteger(lexical); err != nil {
		return err
	}
	if strings.HasPrefix(lexical, "-") {
		for i := 1; i < len(lexical); i++ {
			if lexical[i] != '0' {
				return fmt.Errorf("nonNegativeInteger must be >= 0: %s", lexical)
			}
		}
	}
	return nil
}

func ValidateXSDPositiveInteger(lexical string) error {
	if err := ValidateXSDInteger(lexical); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(lexical))
	if perr != nil || n.Sign <= 0 {
		return fmt.Errorf("positiveInteger must be >= 1: %s", lexical)
	}
	return nil
}

func ValidateXSDUnsignedLong(lexical string) error {
	_, err := ParseUnsignedLong([]byte(lexical))
	return err
}

func ValidateXSDUnsignedInt(lexical string) error {
	_, err := ParseUnsignedInt([]byte(lexical))
	return err
}

func ValidateXSDUnsignedShort(lexical string) error {
	_, err := ParseUnsignedShort([]byte(lexical))
	return err
}

func ValidateXSDUnsignedByte(lexical string) error {
	_, err := ParseUnsignedByte([]byte(lexical))
	return err
}

func ValidateXSDNonPositiveInteger(lexical string) error {
	if err := ValidateXSDInteger(lexical); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(lexical))
	if perr != nil || n.Sign > 0 {
		return fmt.Errorf("nonPositiveInteger must be <= 0: %s", lexical)
	}
	return nil
}

func ValidateXSDNegativeInteger(lexical string) error {
	if err := ValidateXSDInteger(lexical); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(lexical))
	if perr != nil || n.Sign >= 0 {
		return fmt.Errorf("negativeInteger must be < 0: %s", lexical)
	}
	return nil
}

func ValidateXSDNormalizedString(lexical string) error {
	if strings.ContainsAny(lexical, "\r\n\t") {
		return fmt.Errorf("normalizedString cannot contain CR, LF, or Tab")
	}
	return nil
}

func ValidateXSDLanguage(lexical string) error {
	if err := ValidateLanguage([]byte(lexical)); err != nil {
		return fmt.Errorf("invalid language format: %s", lexical)
	}
	return nil
}

func ValidateXSDDuration(lexical string) error {
	_, err := ParseDuration(lexical)
	return err
}

func ValidateXSDDateTime(lexical string) error {
	_, err := ParseDateTime([]byte(lexical))
	return err
}

func ValidateXSDDate(lexical string) error {
	_, err := ParseDate([]byte(lexical))
	return err
}

func ValidateXSDTime(lexical string) error {
	_, err := ParseTime([]byte(lexical))
	return err
}

func ValidateXSDGYear(lexical string) error {
	_, err := ParseGYear([]byte(lexical))
	return err
}

func ValidateXSDGYearMonth(lexical string) error {
	_, err := ParseGYearMonth([]byte(lexical))
	return err
}

func ValidateXSDGMonth(lexical string) error {
	_, err := ParseGMonth([]byte(lexical))
	return err
}

func ValidateXSDGMonthDay(lexical string) error {
	_, err := ParseGMonthDay([]byte(lexical))
	return err
}

func ValidateXSDGDay(lexical string) error {
	_, err := ParseGDay([]byte(lexical))
	return err
}

func ValidateXSDHexBinary(lexical string) error {
	_, err := ParseHexBinary([]byte(lexical))
	return err
}

func ValidateXSDBase64Binary(lexical string) error {
	_, err := ParseBase64Binary([]byte(lexical))
	return err
}

func ValidateXSDToken(lexical string) error {
	return ValidateToken([]byte(lexical))
}

func ValidateXSDName(lexical string) error {
	return ValidateName([]byte(lexical))
}

func ValidateXSDNCName(lexical string) error {
	return ValidateNCName([]byte(lexical))
}

func ValidateXSDNMTOKEN(lexical string) error {
	return ValidateNMTOKEN([]byte(lexical))
}

func ValidateXSDAnyURI(lexical string) error {
	return ValidateAnyURI([]byte(lexical))
}

func ValidateXSDQName(lexical string) error {
	return ValidateQName([]byte(lexical))
}
