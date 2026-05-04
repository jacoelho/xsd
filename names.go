package xsd

type namespaceID uint32
type localNameID uint32

type qName struct {
	Namespace namespaceID
	Local     localNameID
}

const (
	emptyNamespaceURI = ""
	xsdNamespaceURI   = "http://www.w3.org/2001/XMLSchema"
	xsiNamespaceURI   = "http://www.w3.org/2001/XMLSchema-instance"
	xmlNamespaceURI   = "http://www.w3.org/XML/1998/namespace"
	xlinkNamespaceURI = "http://www.w3.org/1999/xlink"
	xmlnsNamespaceURI = "http://www.w3.org/2000/xmlns/"
)

type nameTable struct {
	limitErr   error
	nsIndex    map[string]namespaceID
	localIndex map[string]localNameID
	namespaces []string
	locals     []string
	maxNames   int
}

func newNameTable(maxNames int) nameTable {
	n := nameTable{
		nsIndex:    make(map[string]namespaceID),
		localIndex: make(map[string]localNameID),
		maxNames:   maxNames,
	}
	n.InternNamespace(emptyNamespaceURI)
	n.InternNamespace(xsdNamespaceURI)
	n.InternNamespace(xsiNamespaceURI)
	n.InternNamespace(xmlNamespaceURI)
	n.InternNamespace(xlinkNamespaceURI)
	n.InternNamespace(xmlnsNamespaceURI)
	n.InternQName(xsiNamespaceURI, "type")
	n.InternQName(xsiNamespaceURI, "nil")
	n.InternQName(xsiNamespaceURI, "schemaLocation")
	n.InternQName(xsiNamespaceURI, "noNamespaceSchemaLocation")
	return n
}

func (n *nameTable) InternNamespace(uri string) namespaceID {
	if id, ok := n.nsIndex[uri]; ok {
		return id
	}
	id := namespaceID(len(n.namespaces))
	n.namespaces = append(n.namespaces, uri)
	n.nsIndex[uri] = id
	n.checkLimit()
	return id
}

func (n *nameTable) InternLocal(local string) localNameID {
	if id, ok := n.localIndex[local]; ok {
		return id
	}
	id := localNameID(len(n.locals))
	n.locals = append(n.locals, local)
	n.localIndex[local] = id
	n.checkLimit()
	return id
}

func (n *nameTable) InternQName(ns, local string) qName {
	return qName{Namespace: n.InternNamespace(ns), Local: n.InternLocal(local)}
}

func (n *nameTable) checkLimit() {
	if n.maxNames <= 0 || n.limitErr != nil {
		return
	}
	if len(n.namespaces)+len(n.locals) > n.maxNames {
		n.limitErr = schemaCompile(ErrSchemaLimit, "schema name limit exceeded")
	}
}

func (n nameTable) LookupNamespace(uri string) (namespaceID, bool) {
	id, ok := n.nsIndex[uri]
	return id, ok
}

func (n nameTable) LookupLocal(local string) (localNameID, bool) {
	id, ok := n.localIndex[local]
	return id, ok
}

func (n nameTable) LookupQName(ns, local string) (qName, bool) {
	nsID, ok := n.LookupNamespace(ns)
	if !ok {
		return qName{}, false
	}
	localID, ok := n.LookupLocal(local)
	if !ok {
		return qName{}, false
	}
	return qName{Namespace: nsID, Local: localID}, true
}

func (n nameTable) Namespace(id namespaceID) string {
	i := int(id)
	if i < 0 || i >= len(n.namespaces) {
		return ""
	}
	return n.namespaces[i]
}

func (n nameTable) Local(id localNameID) string {
	i := int(id)
	if i < 0 || i >= len(n.locals) {
		return ""
	}
	return n.locals[i]
}

func (n nameTable) Format(q qName) string {
	ns := n.Namespace(q.Namespace)
	local := n.Local(q.Local)
	if ns == "" {
		return local
	}
	return "{" + ns + "}" + local
}

type runtimeName struct {
	NS    string
	Local string
	Name  qName
	Known bool
}
