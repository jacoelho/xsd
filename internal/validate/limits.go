package validate

import "github.com/jacoelho/xsd/xsderrors"

// ElementLimitInput reports per-start-element resource counters.
type ElementLimitInput struct {
	Context        StartContext
	Depth          int
	MaxDepth       int
	AttributeCount int
	MaxAttributes  int
}

// ValidateElementLimits enforces depth and per-element attribute count limits.
func ValidateElementLimits(in ElementLimitInput) error {
	if in.MaxDepth > 0 && in.Depth > in.MaxDepth {
		return validation(in.Context, xsderrors.CodeValidationLimit, "instance depth limit exceeded")
	}
	if in.MaxAttributes > 0 && in.AttributeCount > in.MaxAttributes {
		return validation(in.Context, xsderrors.CodeValidationLimit, "instance attribute limit exceeded")
	}
	return nil
}

// TextLimitInput reports retained character-data size before appending a token.
type TextLimitInput struct {
	Context      StartContext
	CurrentBytes int
	AppendBytes  int
	MaxBytes     int64
}

// ValidateTextLimit enforces the retained instance character-data byte limit.
func ValidateTextLimit(in TextLimitInput) error {
	if in.MaxBytes <= 0 {
		return nil
	}
	if int64(in.CurrentBytes) > in.MaxBytes-int64(in.AppendBytes) {
		return validation(in.Context, xsderrors.CodeValidationLimit, "instance text byte limit exceeded")
	}
	return nil
}
