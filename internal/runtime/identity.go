package runtime

// IdentityConstraint defines an exported type.
type IdentityConstraint struct {
	Name        SymbolID
	Category    ICCategory
	SelectorOff uint32
	SelectorLen uint32
	FieldOff    uint32
	FieldLen    uint32
	Referenced  ICID
}

// ICCategory defines an exported type.
type ICCategory uint8

const (
	// ICUnique is an exported constant.
	ICUnique ICCategory = iota
	// ICKey is an exported constant.
	ICKey
	// ICKeyRef is an exported constant.
	ICKeyRef
)

// Op defines an exported type.
type Op uint8

const (
	// OpRootSelf is an exported constant.
	OpRootSelf Op = iota
	// OpSelf is an exported constant.
	OpSelf
	// OpDescend is an exported constant.
	OpDescend
	// OpChildName is an exported constant.
	OpChildName
	// OpChildAny is an exported constant.
	OpChildAny
	// OpChildNSAny is an exported constant.
	OpChildNSAny
	// OpAttrName is an exported constant.
	OpAttrName
	// OpAttrAny is an exported constant.
	OpAttrAny
	// OpAttrNSAny is an exported constant.
	OpAttrNSAny
	// OpUnionSplit is an exported constant.
	OpUnionSplit
)

// PathOp defines an exported type.
type PathOp struct {
	Op  Op
	Sym SymbolID
	NS  NamespaceID
}

// PathProgram defines an exported type.
type PathProgram struct {
	Ops []PathOp
}
