package xmlstream

// Limit identifies which Limits field a Reader failure exceeded. The set is
// closed and mirrors the Limits fields the Reader enforces on input; consumers
// map every Limit to their own option name in one exhaustive switch, so a new
// Limit is a compile-time (exhaustive linter) error in each consumer rather
// than a silent syntax error.
type Limit uint8

const (
	// LimitInputBytes is Limits.MaxInputBytes.
	LimitInputBytes Limit = iota + 1
	// LimitTokenBytes is Limits.MaxTokenBytes.
	LimitTokenBytes
	// LimitAttrs is Limits.MaxAttrs.
	LimitAttrs
	// LimitDepth is Limits.MaxDepth.
	LimitDepth
)

// LimitOf reports which Limit err exceeded. Error.Kind is ErrorLimit exactly
// when LimitOf reports true. An error returned by the caller's reader is
// never classified, whatever its chain contains.
func LimitOf(err error) (Limit, bool) {
	switch {
	case originatesSentinel(err, errXMLInputLimit):
		return LimitInputBytes, true
	case originatesSentinel(err, errXMLTokenLimit):
		return LimitTokenBytes, true
	case originatesSentinel(err, errXMLAttributeLimit):
		return LimitAttrs, true
	case originatesSentinel(err, errXMLDepthLimit):
		return LimitDepth, true
	}
	return 0, false
}
