package xmltext

// Kind identifies the syntactic kind of an XML token.
type Kind byte

const (
	KindNone Kind = iota
	KindStartElement
	KindEndElement
	KindCharData
	KindComment
	KindPI
	KindDirective
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
