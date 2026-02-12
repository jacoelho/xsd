package validator

type attrClass uint8

const (
	attrClassOther attrClass = iota
	attrClassXsiKnown
	attrClassXsiUnknown
	attrClassXML
)

type xsiAttrRole uint8

const (
	xsiAttrNone xsiAttrRole = iota
	xsiAttrType
	xsiAttrNil
	xsiAttrSchemaLocation
	xsiAttrNoNamespaceSchemaLocation
)

type attrClassification struct {
	duplicateErr error
	classes      []attrClass
	xsiType      []byte
	xsiNil       []byte
}
