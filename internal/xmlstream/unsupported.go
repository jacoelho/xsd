package xmlstream

import "errors"

// UnsupportedFeature names a well-formed XML feature this reader rejects
// because the project contract excludes it. The set is closed: consumers map
// it with exhaustive switches, so a new feature fails lint at every consumer.
type UnsupportedFeature uint8

const (
	// UnsupportedDTD reports a document type declaration.
	UnsupportedDTD UnsupportedFeature = iota + 1
	// UnsupportedNonUTF8 reports a declared or detected non-UTF-8 encoding.
	UnsupportedNonUTF8
	// UnsupportedXMLVersion reports an XML version other than 1.0. The error
	// text names the version and is the consumer-visible message.
	UnsupportedXMLVersion
	// UnsupportedEntityReference reports a reference to an undeclared general
	// entity. Without DTD support no general entity can be declared.
	UnsupportedEntityReference
)

// UnsupportedFeatureOf classifies err, as returned by this package's readers
// and decoders. Error.Kind is ErrorUnsupported exactly when it reports true.
func UnsupportedFeatureOf(err error) (UnsupportedFeature, bool) {
	switch {
	case errors.Is(err, errUnsupportedDTD):
		return UnsupportedDTD, true
	case errors.Is(err, errUnsupportedNonUTF8):
		return UnsupportedNonUTF8, true
	case errors.Is(err, errUnsupportedEntityReference):
		return UnsupportedEntityReference, true
	}
	if _, ok := errors.AsType[unsupportedXMLVersionError](err); ok {
		return UnsupportedXMLVersion, true
	}
	return 0, false
}
