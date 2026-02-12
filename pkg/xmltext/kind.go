package xmltext

// Kind identifies the syntactic kind of an XML token.
type Kind byte

const (
	// KindNone is an exported constant.
	KindNone Kind = iota
	// KindStartElement is an exported constant.
	KindStartElement
	// KindEndElement is an exported constant.
	KindEndElement
	// KindCharData is an exported constant.
	KindCharData
	// KindComment is an exported constant.
	KindComment
	// KindPI is an exported constant.
	KindPI
	// KindDirective is an exported constant.
	KindDirective
	// KindCDATA is an exported constant.
	KindCDATA
)

// String returns a stable name for the kind, suitable for debugging.
func (k Kind) String() string {
	switch k {
	case KindNone:
		return "None"
	case KindStartElement:
		return "StartElement"
	case KindEndElement:
		return "EndElement"
	case KindCharData:
		return "CharData"
	case KindComment:
		return "Comment"
	case KindPI:
		return "PI"
	case KindDirective:
		return "Directive"
	case KindCDATA:
		return "CDATA"
	default:
		return "Unknown"
	}
}
