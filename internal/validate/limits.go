package validate

import "github.com/jacoelho/xsd/xsderrors"

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
