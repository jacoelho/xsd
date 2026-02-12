package validator

type nsDecl struct {
	prefixOff  uint32
	prefixLen  uint32
	nsOff      uint32
	nsLen      uint32
	prefixHash uint64
}

type prefixEntry struct {
	hash      uint64
	prefixOff uint32
	prefixLen uint32
	nsOff     uint32
	nsLen     uint32
	ok        bool
}
