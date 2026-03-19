package valruntime

// AttributeOptions returns the value-validation request used for attribute
// validation.
func AttributeOptions(requireCanonical, storeValue bool) Options {
	return Options{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: requireCanonical,
		StoreValue:       storeValue,
		NeedKey:          requireCanonical,
	}
}

// TextOptions returns the value-validation request used for simple-content
// text validation.
func TextOptions(requireCanonical, needKey, storeValue bool) Options {
	return Options{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: requireCanonical,
		StoreValue:       storeValue,
		NeedKey:          needKey,
	}
}

// MemberLookupOptions returns the canonical member-lookup request used when
// unions need a second pass to identify the selected member.
func MemberLookupOptions() Options {
	return Options{
		ApplyWhitespace:  true,
		RequireCanonical: true,
	}
}

// ListItemOptions returns the request used to validate one canonicalized list
// item.
func ListItemOptions(parent Options, needKey bool) Options {
	parent.ApplyWhitespace = false
	parent.TrackIDs = false
	parent.RequireCanonical = true
	parent.StoreValue = false
	parent.NeedKey = needKey
	return parent
}

// ListNoCanonicalItemOptions returns the request used to validate one list item
// on the no-canonical path.
func ListNoCanonicalItemOptions(parent Options) Options {
	parent.ApplyWhitespace = false
	parent.TrackIDs = false
	parent.RequireCanonical = false
	parent.StoreValue = false
	parent.NeedKey = false
	return parent
}

// UnionMemberOptions returns the request used to validate one union member.
func UnionMemberOptions(parent Options, applyWhitespace, needKey bool) Options {
	parent.ApplyWhitespace = applyWhitespace
	parent.TrackIDs = false
	parent.RequireCanonical = true
	parent.StoreValue = false
	parent.NeedKey = needKey
	return parent
}

// TextNeedsMetrics reports whether text validation must retain value metrics.
func TextNeedsMetrics(opts Options) bool {
	return opts.StoreValue || opts.NeedKey
}
