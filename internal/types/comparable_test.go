package types

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestComparable_Decimal(t *testing.T) {
	rat1, _ := ParseDecimal("123.456")
	rat2, _ := ParseDecimal("789.012")
	rat3, _ := ParseDecimal("123.456")

	decimalType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "decimal"},
	}
	comp1 := ComparableDec{Value: rat1, Typ: decimalType}
	comp2 := ComparableDec{Value: rat2, Typ: decimalType}
	comp3 := ComparableDec{Value: rat3, Typ: decimalType}

	// test Compare
	cmp, err := comp1.Compare(comp2)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("rat1 should be less than rat2")
	}
	cmp, err = comp1.Compare(comp3)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp != 0 {
		t.Error("rat1 should equal rat3")
	}
	cmp, err = comp2.Compare(comp1)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp <= 0 {
		t.Error("rat2 should be greater than rat1")
	}

}

func TestComparable_Integer(t *testing.T) {
	int1, _ := ParseInteger("123")
	int2, _ := ParseInteger("789")
	int3, _ := ParseInteger("123")

	integerType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "integer"},
	}
	comp1 := ComparableInt{Value: int1, Typ: integerType}
	comp2 := ComparableInt{Value: int2, Typ: integerType}
	comp3 := ComparableInt{Value: int3, Typ: integerType}

	// test Compare
	cmp, err := comp1.Compare(comp2)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("int1 should be less than int2")
	}
	cmp, err = comp1.Compare(comp3)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp != 0 {
		t.Error("int1 should equal int3")
	}
	cmp, err = comp2.Compare(comp1)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp <= 0 {
		t.Error("int2 should be greater than int1")
	}

}

// TestComparable_CrossTypeNumeric tests that numeric types can be compared across type boundaries
// This is required for XSD facet validation where integer values may need to be compared
// with decimal facet values (or vice versa) since integers are a subset of decimals.
func TestComparable_CrossTypeNumeric(t *testing.T) {
	decimalType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "decimal"},
	}
	integerType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "integer"},
	}

	int100, _ := ParseInteger("100")
	decimal100, _ := ParseDecimal("100.0")
	compInt := ComparableInt{Value: int100, Typ: integerType}
	compDecimal := ComparableDec{Value: decimal100, Typ: decimalType}

	t.Run("integer_100_equals_decimal_100.0", func(t *testing.T) {
		cmp, err := compInt.Compare(compDecimal)
		if err != nil {
			t.Fatalf("Compare(integer 100, decimal 100.0) error = %v, want nil", err)
		}
		if cmp != 0 {
			t.Errorf("Compare(integer 100, decimal 100.0) = %d, want 0 (equal)", cmp)
		}
	})

	t.Run("decimal_100.0_equals_integer_100", func(t *testing.T) {
		cmp, err := compDecimal.Compare(compInt)
		if err != nil {
			t.Fatalf("Compare(decimal 100.0, integer 100) error = %v, want nil", err)
		}
		if cmp != 0 {
			t.Errorf("Compare(decimal 100.0, integer 100) = %d, want 0 (equal)", cmp)
		}
	})

	t.Run("integer_50_less_than_decimal_100.0", func(t *testing.T) {
		int50, _ := ParseInteger("50")
		compInt50 := ComparableInt{Value: int50, Typ: integerType}
		cmp, err := compInt50.Compare(compDecimal)
		if err != nil {
			t.Fatalf("Compare(integer 50, decimal 100.0) error = %v, want nil", err)
		}
		if cmp >= 0 {
			t.Errorf("Compare(integer 50, decimal 100.0) = %d, want < 0", cmp)
		}
	})

	t.Run("decimal_150.0_greater_than_integer_100", func(t *testing.T) {
		decimal150, _ := ParseDecimal("150.0")
		compDecimal150 := ComparableDec{Value: decimal150, Typ: decimalType}
		cmp, err := compDecimal150.Compare(compInt)
		if err != nil {
			t.Fatalf("Compare(decimal 150.0, integer 100) error = %v, want nil", err)
		}
		if cmp <= 0 {
			t.Errorf("Compare(decimal 150.0, integer 100) = %d, want > 0", cmp)
		}
	})

	t.Run("integer_100_greater_than_decimal_99.9", func(t *testing.T) {
		decimal999, _ := ParseDecimal("99.9")
		compDecimal999 := ComparableDec{Value: decimal999, Typ: decimalType}
		cmp, err := compInt.Compare(compDecimal999)
		if err != nil {
			t.Fatalf("Compare(integer 100, decimal 99.9) error = %v, want nil", err)
		}
		if cmp <= 0 {
			t.Errorf("Compare(integer 100, decimal 99.9) = %d, want > 0", cmp)
		}
	})

	t.Run("decimal_100.1_greater_than_integer_100", func(t *testing.T) {
		decimal1001, _ := ParseDecimal("100.1")
		compDecimal1001 := ComparableDec{Value: decimal1001, Typ: decimalType}
		cmp, err := compDecimal1001.Compare(compInt)
		if err != nil {
			t.Fatalf("Compare(decimal 100.1, integer 100) error = %v, want nil", err)
		}
		if cmp <= 0 {
			t.Errorf("Compare(decimal 100.1, integer 100) = %d, want > 0", cmp)
		}
	})
}

func TestComparable_Time(t *testing.T) {
	time1, _ := ParseDateTime("2001-10-26T21:32:52")
	time2, _ := ParseDateTime("2002-10-26T21:32:52")
	time3, _ := ParseDateTime("2001-10-26T21:32:52")

	dateTimeType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "dateTime"},
	}
	comp1 := ComparableTime{Value: time1, Typ: dateTimeType}
	comp2 := ComparableTime{Value: time2, Typ: dateTimeType}
	comp3 := ComparableTime{Value: time3, Typ: dateTimeType}

	// test Compare
	cmp, err := comp1.Compare(comp2)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("time1 should be before time2")
	}
	cmp, err = comp1.Compare(comp3)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp != 0 {
		t.Error("time1 should equal time3")
	}
	cmp, err = comp2.Compare(comp1)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp <= 0 {
		t.Error("time2 should be after time1")
	}

}

func TestComparable_TimeTimezoneMismatch(t *testing.T) {
	timeZ, _ := ParseDateTime("2001-10-26T21:32:52Z")
	timeNoTZ, _ := ParseDateTime("2001-10-26T21:32:52")

	dateTimeType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "dateTime"},
	}
	compZ := ComparableTime{Value: timeZ, Typ: dateTimeType, HasTimezone: true}
	compNo := ComparableTime{Value: timeNoTZ, Typ: dateTimeType, HasTimezone: false}

	if _, err := compZ.Compare(compNo); err == nil {
		t.Fatalf("expected timezone mismatch comparison error")
	}
}

func TestComparable_Float64(t *testing.T) {
	doubleType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "double"},
	}
	// test normal values
	comp1 := ComparableFloat64{Value: 123.456, Typ: doubleType}
	comp2 := ComparableFloat64{Value: 789.012, Typ: doubleType}
	comp3 := ComparableFloat64{Value: 123.456, Typ: doubleType}

	cmp, err := comp1.Compare(comp2)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("float1 should be less than float2")
	}
	cmp, err = comp1.Compare(comp3)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp != 0 {
		t.Error("float1 should equal float3")
	}

	// test INF
	infComp := ComparableFloat64{Value: math.Inf(1), Typ: doubleType}
	negInfComp := ComparableFloat64{Value: math.Inf(-1), Typ: doubleType}
	normalComp := ComparableFloat64{Value: 100.0, Typ: doubleType}

	// INF > normal
	cmp, err = infComp.Compare(normalComp)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp <= 0 {
		t.Error("INF should be greater than normal value")
	}

	// -INF < normal
	cmp, err = negInfComp.Compare(normalComp)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("-INF should be less than normal value")
	}

	// INF > -INF
	cmp, err = infComp.Compare(negInfComp)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp <= 0 {
		t.Error("INF should be greater than -INF")
	}

	// test NaN - should return error or special value
	nanComp := ComparableFloat64{Value: math.NaN(), Typ: doubleType}
	_, err = nanComp.Compare(infComp)
	if err == nil {
		t.Error("NaN comparison should return error")
	}
}

func TestParseDurationToTimeDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		errMsg  string
		want    time.Duration
		wantErr bool
	}{
		// valid pure day/time durations
		{
			name:  "days only",
			input: "P1D",
			want:  24 * time.Hour,
		},
		{
			name:  "hours only",
			input: "PT1H",
			want:  1 * time.Hour,
		},
		{
			name:  "minutes only",
			input: "PT1M",
			want:  1 * time.Minute,
		},
		{
			name:  "seconds only",
			input: "PT1S",
			want:  1 * time.Second,
		},
		{
			name:  "days and hours",
			input: "P1DT2H",
			want:  24*time.Hour + 2*time.Hour,
		},
		{
			name:  "full duration",
			input: "P1DT2H3M4S",
			want:  24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second,
		},
		{
			name:  "fractional seconds",
			input: "PT1.5S",
			want:  1500 * time.Millisecond,
		},
		{
			name:  "negative duration",
			input: "-P1DT2H",
			want:  -(24*time.Hour + 2*time.Hour),
		},
		{
			name:  "zero duration",
			input: "PT0S",
			want:  0,
		},
		{
			name:  "zero duration with days",
			input: "P0D",
			want:  0,
		},
		{
			name:  "large duration",
			input: "P365DT23H59M59S",
			want:  365*24*time.Hour + 23*time.Hour + 59*time.Minute + 59*time.Second,
		},
		{
			name:    "duration overflow days",
			input:   "P1000000D",
			wantErr: true,
			errMsg:  "duration too large",
		},
		// invalid durations - with years/months
		{
			name:    "with years",
			input:   "P1Y",
			wantErr: true,
			errMsg:  "years or months",
		},
		{
			name:    "with months",
			input:   "P1M",
			wantErr: true,
			errMsg:  "years or months",
		},
		{
			name:    "with years and days",
			input:   "P1Y2D",
			wantErr: true,
			errMsg:  "years or months",
		},
		{
			name:    "with months and hours",
			input:   "P1MT2H",
			wantErr: true,
			errMsg:  "years or months",
		},
		// invalid formats
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "empty duration",
		},
		{
			name:    "missing P",
			input:   "1D",
			wantErr: true,
			errMsg:  "must start with P",
		},
		{
			name:    "no components",
			input:   "P",
			wantErr: true,
			errMsg:  "at least one component",
		},
		{
			name:    "no components with T",
			input:   "PT",
			wantErr: true,
			errMsg:  "at least one component",
		},
		{
			name:    "multiple T separators",
			input:   "PT1HT2M",
			wantErr: true,
			errMsg:  "multiple T separators",
		},
		{
			name:    "invalid year value",
			input:   "P999999999999999999999Y",
			wantErr: true,
			errMsg:  "too large",
		},
		{
			name:    "second value too large",
			input:   "PT999999999999999999999S",
			wantErr: true,
			errMsg:  "second value too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDurationToTimeDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDurationToTimeDuration(%q) expected error, got nil", tt.input)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseDurationToTimeDuration(%q) error = %v, want error containing %q", tt.input, err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseDurationToTimeDuration(%q) error = %v, want nil", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDurationToTimeDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestComparableDuration(t *testing.T) {
	// test valid durations
	dur1, err := ParseDurationToTimeDuration("P1DT2H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	dur2, err := ParseDurationToTimeDuration("P2DT4H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	dur3, err := ParseDurationToTimeDuration("P1DT2H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}

	durationType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "duration"},
	}
	comp1 := ComparableDuration{Value: dur1, Typ: durationType}
	comp2 := ComparableDuration{Value: dur2, Typ: durationType}
	comp3 := ComparableDuration{Value: dur3, Typ: durationType}

	// test Compare
	cmp, err := comp1.Compare(comp2)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("dur1 should be less than dur2")
	}

	cmp, err = comp1.Compare(comp3)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp != 0 {
		t.Error("dur1 should equal dur3")
	}

	cmp, err = comp2.Compare(comp1)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp <= 0 {
		t.Error("dur2 should be greater than dur1")
	}

	// test negative durations
	negDur, err := ParseDurationToTimeDuration("-P1DT2H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	negComp := ComparableDuration{Value: negDur, Typ: durationType}

	cmp, err = negComp.Compare(comp1)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Error("negative duration should be less than positive")
	}
}

func TestComparableXSDDuration_Compare(t *testing.T) {
	durationType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "duration"},
	}

	left, err := ParseXSDDuration("PT26H")
	if err != nil {
		t.Fatalf("ParseXSDDuration() error = %v", err)
	}
	right, err := ParseXSDDuration("P1DT2H")
	if err != nil {
		t.Fatalf("ParseXSDDuration() error = %v", err)
	}

	cmp, err := ComparableXSDDuration{Value: left, Typ: durationType}.Compare(
		ComparableXSDDuration{Value: right, Typ: durationType},
	)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp != 0 {
		t.Errorf("PT26H should equal P1DT2H, cmp=%d", cmp)
	}

	longer, err := ParseXSDDuration("P32D")
	if err != nil {
		t.Fatalf("ParseXSDDuration() error = %v", err)
	}
	month, err := ParseXSDDuration("P1M")
	if err != nil {
		t.Fatalf("ParseXSDDuration() error = %v", err)
	}
	cmp, err = ComparableXSDDuration{Value: month, Typ: durationType}.Compare(
		ComparableXSDDuration{Value: longer, Typ: durationType},
	)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if cmp >= 0 {
		t.Errorf("P1M should be less than P32D, cmp=%d", cmp)
	}
}

func TestComparableXSDDuration_CarryBorrow(t *testing.T) {
	durationType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "duration"},
	}
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{
			name: "seconds to minutes carry",
			a:    "P1M0DT0H1M0S",
			b:    "P1M0DT0H0M60S",
			want: 0,
		},
		{
			name: "minutes to hours carry",
			a:    "P1M0DT1H0M0S",
			b:    "P1M0DT0H60M0S",
			want: 0,
		},
		{
			name: "negative seconds ordering",
			a:    "-P1M0DT0H0M1S",
			b:    "-P1M0DT0H0M2S",
			want: 1,
		},
		{
			name: "fractional seconds ordering",
			a:    "P1M0DT0H0M0.5S",
			b:    "P1M0DT0H0M0.4S",
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, err := ParseXSDDuration(tt.a)
			if err != nil {
				t.Fatalf("ParseXSDDuration(%q) error = %v", tt.a, err)
			}
			right, err := ParseXSDDuration(tt.b)
			if err != nil {
				t.Fatalf("ParseXSDDuration(%q) error = %v", tt.b, err)
			}
			cmp, err := ComparableXSDDuration{Value: left, Typ: durationType}.Compare(
				ComparableXSDDuration{Value: right, Typ: durationType},
			)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}
			switch {
			case tt.want == 0 && cmp != 0:
				t.Fatalf("compare(%q,%q) = %d, want 0", tt.a, tt.b, cmp)
			case tt.want < 0 && cmp >= 0:
				t.Fatalf("compare(%q,%q) = %d, want <0", tt.a, tt.b, cmp)
			case tt.want > 0 && cmp <= 0:
				t.Fatalf("compare(%q,%q) = %d, want >0", tt.a, tt.b, cmp)
			}
		})
	}
}

func TestComparableXSDDuration_Indeterminate(t *testing.T) {
	durationType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "duration"},
	}

	month, err := ParseXSDDuration("P1M")
	if err != nil {
		t.Fatalf("ParseXSDDuration() error = %v", err)
	}
	days, err := ParseXSDDuration("P30D")
	if err != nil {
		t.Fatalf("ParseXSDDuration() error = %v", err)
	}

	_, err = ComparableXSDDuration{Value: month, Typ: durationType}.Compare(
		ComparableXSDDuration{Value: days, Typ: durationType},
	)
	if !errors.Is(err, errIndeterminateDurationComparison) {
		t.Fatalf("expected indeterminate comparison error, got %v", err)
	}
}

func TestComparableXSDDuration_StringCanonical(t *testing.T) {
	durationType := mustBuiltinSimpleType(t, TypeNameDuration)

	tests := []struct {
		input string
		want  string
	}{
		{input: "P0D", want: "PT0S"},
		{input: "PT0S", want: "PT0S"},
		{input: "-P0D", want: "PT0S"},
		{input: "-PT0S", want: "PT0S"},
		{input: "PT0.000001S", want: "PT0.000001S"},
		{input: "PT123456.789S", want: "PT123456.789S"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dur, err := ParseXSDDuration(tt.input)
			if err != nil {
				t.Fatalf("ParseXSDDuration() error = %v", err)
			}
			got := ComparableXSDDuration{Value: dur, Typ: durationType}.String()
			if got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComparableDuration_Unwrap(t *testing.T) {
	durationType := &SimpleType{
		QName: QName{Namespace: XSDNamespace, Local: "duration"},
	}
	dur, err := ParseDurationToTimeDuration("P1DT2H")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	comp := ComparableDuration{Value: dur, Typ: durationType}

	unwrapped := comp.Unwrap()
	if unwrapped != dur {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, dur)
	}
}
