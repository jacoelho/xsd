package validator

// valueOptions controls whitespace, canonicalization, storage, and key derivation
// for one value-validation request.
type valueOptions struct {
	ApplyWhitespace  bool
	TrackIDs         bool
	RequireCanonical bool
	StoreValue       bool
	NeedKey          bool
}
