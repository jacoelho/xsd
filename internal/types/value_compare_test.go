package types

import "testing"

func TestValuesEqual_NaN(t *testing.T) {
	floatType := mustBuiltinSimpleType(t, TypeNameFloat)
	left, err := floatType.ParseValue("NaN")
	if err != nil {
		t.Fatalf("ParseValue(NaN) error = %v", err)
	}
	right, err := floatType.ParseValue("NaN")
	if err != nil {
		t.Fatalf("ParseValue(NaN) error = %v", err)
	}
	if !ValuesEqual(left, right) {
		t.Fatalf("expected NaN values to be equal")
	}
	nonNaN, err := floatType.ParseValue("1.0")
	if err != nil {
		t.Fatalf("ParseValue(1.0) error = %v", err)
	}
	if ValuesEqual(left, nonNaN) {
		t.Fatalf("expected NaN to differ from non-NaN value")
	}

	doubleType := mustBuiltinSimpleType(t, TypeNameDouble)
	leftDouble, err := doubleType.ParseValue("NaN")
	if err != nil {
		t.Fatalf("ParseValue(NaN) error = %v", err)
	}
	rightDouble, err := doubleType.ParseValue("NaN")
	if err != nil {
		t.Fatalf("ParseValue(NaN) error = %v", err)
	}
	if !ValuesEqual(leftDouble, rightDouble) {
		t.Fatalf("expected double NaN values to be equal")
	}
}

func TestValuesEqual_DurationBinary(t *testing.T) {
	durationType := mustBuiltinSimpleType(t, TypeNameDuration)
	leftDur, err := durationType.ParseValue("P1D")
	if err != nil {
		t.Fatalf("ParseValue(P1D) error = %v", err)
	}
	rightDur, err := durationType.ParseValue("PT24H")
	if err != nil {
		t.Fatalf("ParseValue(PT24H) error = %v", err)
	}
	if !ValuesEqual(leftDur, rightDur) {
		t.Fatalf("expected duration values to be equal")
	}

	hexType := mustBuiltinSimpleType(t, TypeNameHexBinary)
	leftHex, err := hexType.ParseValue("0A")
	if err != nil {
		t.Fatalf("ParseValue(0A) error = %v", err)
	}
	rightHex, err := hexType.ParseValue("0a")
	if err != nil {
		t.Fatalf("ParseValue(0a) error = %v", err)
	}
	if !ValuesEqual(leftHex, rightHex) {
		t.Fatalf("expected hexBinary values to be equal")
	}

	base64Type := mustBuiltinSimpleType(t, TypeNameBase64Binary)
	leftB64, err := base64Type.ParseValue("AQID")
	if err != nil {
		t.Fatalf("ParseValue(AQID) error = %v", err)
	}
	rightB64, err := base64Type.ParseValue("A Q I D")
	if err != nil {
		t.Fatalf("ParseValue(A Q I D) error = %v", err)
	}
	if !ValuesEqual(leftB64, rightB64) {
		t.Fatalf("expected base64Binary values to be equal")
	}
}

func TestValuesEqual_DateTimeTimezonePresence(t *testing.T) {
	dateTimeType := mustBuiltinSimpleType(t, TypeNameDateTime)
	noTimezone, err := dateTimeType.ParseValue("2000-01-01T00:00:00")
	if err != nil {
		t.Fatalf("ParseValue(no timezone) error = %v", err)
	}
	withTimezone, err := dateTimeType.ParseValue("2000-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("ParseValue(with timezone) error = %v", err)
	}
	if ValuesEqual(noTimezone, withTimezone) {
		t.Fatalf("expected dateTime values with and without timezone to differ")
	}
}

func TestValuesEqual_TimeTimezoneWrap(t *testing.T) {
	timeType := mustBuiltinSimpleType(t, TypeNameTime)
	left, err := timeType.ParseValue("23:30:00-01:00")
	if err != nil {
		t.Fatalf("ParseValue(23:30:00-01:00) error = %v", err)
	}
	right, err := timeType.ParseValue("00:30:00Z")
	if err != nil {
		t.Fatalf("ParseValue(00:30:00Z) error = %v", err)
	}
	if ValuesEqual(left, right) {
		t.Fatalf("expected time values with different reference dates to differ")
	}
}
