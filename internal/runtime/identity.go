package runtime

// IdentityConstraint stores the compiled selector/field program for one constraint.
type IdentityConstraint struct {
	Name        SymbolID
	Category    ICCategory
	SelectorOff uint32
	SelectorLen uint32
	FieldOff    uint32
	FieldLen    uint32
	Referenced  ICID
}

// ICCategory enumerates identity-constraint categories.
type ICCategory uint8

const (
	ICUnique ICCategory = iota
	ICKey
	ICKeyRef
)

// Op enumerates selector/field path opcodes.
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

// PathOp is one instruction in a compiled path program.
type PathOp struct {
	Op  Op
	Sym SymbolID
	NS  NamespaceID
}

// PathProgram is a compiled selector/field path.
type PathProgram struct {
	Ops []PathOp
}
