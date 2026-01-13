package validator

// KeyState reports whether a key field value is valid.
type KeyState int

const (
	KeyValid KeyState = iota
	KeyInvalid
)
