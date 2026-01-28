package runtime

type IdentityConstraint struct {
	Name        SymbolID
	Category    ICCategory
	SelectorOff uint32
	SelectorLen uint32
	FieldOff    uint32
	FieldLen    uint32
	Referenced  ICID
}

type ICCategory uint8

const (
	ICUnique ICCategory = iota
	ICKey
	ICKeyRef
)

type Op uint8

const (
	OpRootSelf Op = iota
	OpSelf
	OpDescend
	OpChildName
	OpChildAny
	OpChildNSAny
	OpAttrName
	OpAttrAny
	OpAttrNSAny
	OpUnionSplit
)

type PathOp struct {
	Op  Op
	Sym SymbolID
	NS  NamespaceID
}

type PathProgram struct {
	Ops []PathOp
}
