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

type qnameCache struct {
	table       map[qnameKey]types.QName
	recent      [qnameCacheRecentSize]qnameCacheEntry
	recentCount int
	recentIndex int
}

type qnameKey struct {
	namespace string
	local     string
}

const qnameCacheRecentSize = 8

type qnameCacheEntry struct {
	namespace string
	local     string
	qname     types.QName
}

func newQNameCache() *qnameCache {
	return &qnameCache{
		table: make(map[qnameKey]types.QName, 32),
	}
}

func (i *qnameCache) lookupRecent(namespace, local string) (types.QName, bool) {
	for idx := 0; idx < i.recentCount; idx++ {
		entry := i.recent[idx]
		if entry.namespace == namespace && entry.local == local {
			return entry.qname, true
		}
	}
	return types.QName{}, false
}

func (i *qnameCache) rememberRecent(entry qnameCacheEntry) {
	if i.recentCount < qnameCacheRecentSize {
		i.recent[i.recentCount] = entry
		i.recentCount++
		return
	}
	i.recent[i.recentIndex] = entry
	i.recentIndex++
	if i.recentIndex >= qnameCacheRecentSize {
		i.recentIndex = 0
	}
}

func (i *qnameCache) intern(namespace, local string) types.QName {
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

	if cached, ok := i.lookupRecent(namespace, local); ok {
		return cached
	}
	key := qnameKey{namespace: namespace, local: local}
	if cached, ok := i.table[key]; ok {
		i.rememberRecent(qnameCacheEntry{namespace: namespace, local: local, qname: cached})
		return cached
	}

	qname := types.QName{
		Namespace: types.NamespaceURI(namespace),
		Local:     local,
	}
	i.table[key] = qname
	i.rememberRecent(qnameCacheEntry{namespace: namespace, local: local, qname: qname})
	return qname
}
