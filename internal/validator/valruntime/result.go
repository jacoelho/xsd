package valruntime

import "github.com/jacoelho/xsd/internal/runtime"

// Result stores derived key data, union-selected actual type data, and facet progress flags.
type Result struct {
	key             runtime.ValueKey
	actualTypeID    runtime.TypeID
	actualValidator runtime.ValidatorID
	patternChecked  bool
	enumChecked     bool
}

// SetKey stores one derived runtime key.
func (s *Result) SetKey(kind runtime.ValueKind, key []byte) {
	if s == nil {
		return
	}
	s.key.Kind = kind
	s.key.Bytes = key
	s.key.Hash = 0
}

// HasKey reports whether a derived runtime key is present.
func (s *Result) HasKey() bool {
	return s != nil && s.key.Kind != runtime.VKInvalid
}

// Key returns the derived runtime key when present.
func (s *Result) Key() (runtime.ValueKind, []byte, bool) {
	if !s.HasKey() {
		return runtime.VKInvalid, nil, false
	}
	return s.key.Kind, s.key.Bytes, true
}

// SetActual stores the actual selected type and validator for union validation.
func (s *Result) SetActual(typeID runtime.TypeID, validatorID runtime.ValidatorID) {
	if s == nil {
		return
	}
	s.actualTypeID = typeID
	s.actualValidator = validatorID
}

// Actual returns the actual selected type and validator for union validation.
func (s *Result) Actual() (runtime.TypeID, runtime.ValidatorID) {
	if s == nil {
		return 0, 0
	}
	return s.actualTypeID, s.actualValidator
}

// SetPatternChecked stores whether pattern facets were evaluated.
func (s *Result) SetPatternChecked(checked bool) {
	if s == nil {
		return
	}
	s.patternChecked = checked
}

// PatternChecked reports whether pattern facets were evaluated.
func (s *Result) PatternChecked() bool {
	return s != nil && s.patternChecked
}

// SetEnumChecked stores whether enumeration facets were evaluated.
func (s *Result) SetEnumChecked(checked bool) {
	if s == nil {
		return
	}
	s.enumChecked = checked
}

// EnumChecked reports whether enumeration facets were evaluated.
func (s *Result) EnumChecked() bool {
	return s != nil && s.enumChecked
}
