package xsd

type namespaceID uint32
type localNameID uint32

type qName struct {
	Namespace namespaceID
	Local     localNameID
}

func validUint32Index(id uint32, n int) bool {
	if n < 0 {
		return false
	}
	return uint64(id) < uint64(n)
}

const (
	emptyNamespaceID  namespaceID = 0
	emptyNamespaceURI             = ""
	xsdNamespaceURI               = "http://www.w3.org/2001/XMLSchema"
	xsiNamespaceURI               = "http://www.w3.org/2001/XMLSchema-instance"
	xmlNamespaceURI               = "http://www.w3.org/XML/1998/namespace"
	xlinkNamespaceURI             = "http://www.w3.org/1999/xlink"
	xmlnsNamespaceURI             = "http://www.w3.org/2000/xmlns/"
)

type nameTable struct {
	nsIndex    map[string]namespaceID
	localIndex map[string]localNameID
	namespaces []string
	locals     []string
	maxNames   int
}

func newNameTable(maxNames int) (nameTable, error) {
	n := nameTable{
		nsIndex:    make(map[string]namespaceID, 8),
		localIndex: make(map[string]localNameID, builtinGlobalTypeCount),
		namespaces: make([]string, 0, 8),
		locals:     make([]string, 0, builtinGlobalTypeCount),
		maxNames:   maxNames,
	}
	for _, uri := range []string{
		emptyNamespaceURI,
		xsdNamespaceURI,
		xsiNamespaceURI,
		xmlNamespaceURI,
		xlinkNamespaceURI,
		xmlnsNamespaceURI,
	} {
		if _, err := n.InternNamespace(uri); err != nil {
			return nameTable{}, err
		}
	}
	for _, local := range []string{xsiAttrType, xsiAttrNil, xsiAttrSchemaLocation, xsiAttrNoNamespaceSchemaLocation} {
		if _, err := n.InternQName(xsiNamespaceURI, local); err != nil {
			return nameTable{}, err
		}
	}
	return n, nil
}

func (n *nameTable) InternNamespace(uri string) (namespaceID, error) {
	if id, ok := n.nsIndex[uri]; ok {
		return id, nil
	}
	if err := n.checkLimit(1); err != nil {
		return 0, err
	}
	id, err := nextNamespaceID(len(n.namespaces))
	if err != nil {
		return 0, err
	}
	n.namespaces = append(n.namespaces, uri)
	n.nsIndex[uri] = id
	return id, nil
}

func (n *nameTable) InternLocal(local string) (localNameID, error) {
	if id, ok := n.localIndex[local]; ok {
		return id, nil
	}
	if err := n.checkLimit(1); err != nil {
		return 0, err
	}
	id, err := nextLocalNameID(len(n.locals))
	if err != nil {
		return 0, err
	}
	n.locals = append(n.locals, local)
	n.localIndex[local] = id
	return id, nil
}

func (n *nameTable) InternQName(ns, local string) (qName, error) {
	nsID, nsOK := n.nsIndex[ns]
	localID, localOK := n.localIndex[local]
	need := 0
	if !nsOK {
		need++
	}
	if !localOK {
		need++
	}
	if err := n.checkLimit(need); err != nil {
		return qName{}, err
	}
	if !nsOK {
		var err error
		nsID, err = nextNamespaceID(len(n.namespaces))
		if err != nil {
			return qName{}, err
		}
		n.namespaces = append(n.namespaces, ns)
		n.nsIndex[ns] = nsID
	}
	if !localOK {
		var err error
		localID, err = nextLocalNameID(len(n.locals))
		if err != nil {
			return qName{}, err
		}
		n.locals = append(n.locals, local)
		n.localIndex[local] = localID
	}
	return qName{Namespace: nsID, Local: localID}, nil
}

func (n *nameTable) checkLimit(need int) error {
	if n.maxNames <= 0 || need <= 0 {
		return nil
	}
	if len(n.namespaces)+len(n.locals)+need > n.maxNames {
		return schemaCompile(ErrSchemaLimit, "schema name limit exceeded")
	}
	return nil
}

func (n *nameTable) LookupNamespace(uri string) (namespaceID, bool) {
	id, ok := n.nsIndex[uri]
	return id, ok
}

func (n *nameTable) LookupLocal(local string) (localNameID, bool) {
	id, ok := n.localIndex[local]
	return id, ok
}

func (n *nameTable) LookupQName(ns, local string) (qName, bool) {
	if ns == "" {
		localID, ok := n.LookupLocal(local)
		if !ok {
			return qName{}, false
		}
		return qName{Namespace: emptyNamespaceID, Local: localID}, true
	}
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

func (n *nameTable) Namespace(id namespaceID) string {
	if !validUint32Index(uint32(id), len(n.namespaces)) {
		return ""
	}
	return n.namespaces[id]
}

func (n *nameTable) Local(id localNameID) string {
	if !validUint32Index(uint32(id), len(n.locals)) {
		return ""
	}
	return n.locals[id]
}

func (n *nameTable) Format(q qName) string {
	return formatExpandedName(n.Namespace(q.Namespace), n.Local(q.Local))
}

type runtimeName struct {
	NS    string
	Local string
	Name  qName
	Known bool
}

func (n runtimeName) label() string {
	if n.Known || n.NS == "" {
		return n.Local
	}
	return formatExpandedName(n.NS, n.Local)
}
