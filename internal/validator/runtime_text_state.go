package validator

// TextState tracks character data for the current element.
type TextState struct {
	Off uint32
	Len uint32

	HasText  bool
	HasNonWS bool
}
