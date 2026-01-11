package xsdxml

import "github.com/jacoelho/xsd/internal/types"

var (
	// common XSD namespace QNames (pre-allocated).
	xsNamespace = types.NamespaceURI(XSDNamespace)

	qnameXSSchema         = types.QName{Namespace: xsNamespace, Local: "schema"}
	qnameXSElement        = types.QName{Namespace: xsNamespace, Local: "element"}
	qnameXSComplexType    = types.QName{Namespace: xsNamespace, Local: "complexType"}
	qnameXSSimpleType     = types.QName{Namespace: xsNamespace, Local: "simpleType"}
	qnameXSAttribute      = types.QName{Namespace: xsNamespace, Local: "attribute"}
	qnameXSSequence       = types.QName{Namespace: xsNamespace, Local: "sequence"}
	qnameXSChoice         = types.QName{Namespace: xsNamespace, Local: "choice"}
	qnameXSGroup          = types.QName{Namespace: xsNamespace, Local: "group"}
	qnameXSAttributeGroup = types.QName{Namespace: xsNamespace, Local: "attributeGroup"}
	qnameXSSimpleContent  = types.QName{Namespace: xsNamespace, Local: "simpleContent"}
	qnameXSComplexContent = types.QName{Namespace: xsNamespace, Local: "complexContent"}
	qnameXSExtension      = types.QName{Namespace: xsNamespace, Local: "extension"}
	qnameXSRestriction    = types.QName{Namespace: xsNamespace, Local: "restriction"}
	qnameXSList           = types.QName{Namespace: xsNamespace, Local: "list"}
	qnameXSUnion          = types.QName{Namespace: xsNamespace, Local: "union"}
	qnameXSAny            = types.QName{Namespace: xsNamespace, Local: "any"}
	qnameXSAnyAttribute   = types.QName{Namespace: xsNamespace, Local: "anyAttribute"}
	qnameXSAnnotation     = types.QName{Namespace: xsNamespace, Local: "annotation"}
	qnameXSImport         = types.QName{Namespace: xsNamespace, Local: "import"}
	qnameXSInclude        = types.QName{Namespace: xsNamespace, Local: "include"}
)

type qnameInterner struct {
	table map[qnameKey]types.QName
}

type qnameKey struct {
	namespace string
	local     string
}

func newQNameInterner() *qnameInterner {
	return &qnameInterner{
		table: make(map[qnameKey]types.QName, 32),
	}
}

func (i *qnameInterner) intern(namespace, local string) types.QName {
	if namespace == XSDNamespace {
		switch local {
		case "schema":
			return qnameXSSchema
		case "element":
			return qnameXSElement
		case "complexType":
			return qnameXSComplexType
		case "simpleType":
			return qnameXSSimpleType
		case "attribute":
			return qnameXSAttribute
		case "sequence":
			return qnameXSSequence
		case "choice":
			return qnameXSChoice
		case "group":
			return qnameXSGroup
		case "attributeGroup":
			return qnameXSAttributeGroup
		case "simpleContent":
			return qnameXSSimpleContent
		case "complexContent":
			return qnameXSComplexContent
		case "extension":
			return qnameXSExtension
		case "restriction":
			return qnameXSRestriction
		case "list":
			return qnameXSList
		case "union":
			return qnameXSUnion
		case "any":
			return qnameXSAny
		case "anyAttribute":
			return qnameXSAnyAttribute
		case "annotation":
			return qnameXSAnnotation
		case "import":
			return qnameXSImport
		case "include":
			return qnameXSInclude
		}
	}

	key := qnameKey{namespace: namespace, local: local}
	if cached, ok := i.table[key]; ok {
		return cached
	}

	qname := types.QName{
		Namespace: types.NamespaceURI(namespace),
		Local:     local,
	}
	i.table[key] = qname
	return qname
}
