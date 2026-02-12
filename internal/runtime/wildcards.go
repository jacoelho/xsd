package runtime

// ProcessContents defines an exported type.
type ProcessContents uint8

const (
	// PCStrict is an exported constant.
	PCStrict ProcessContents = iota
	// PCLax is an exported constant.
	PCLax
	// PCSkip is an exported constant.
	PCSkip
)

// NSConstraintKind defines an exported type.
type NSConstraintKind uint8

const (
	// NSAny is an exported constant.
	NSAny NSConstraintKind = iota
	// NSOther is an exported constant.
	NSOther
	// NSEnumeration is an exported constant.
	NSEnumeration
	// NSNotAbsent is an exported constant.
	NSNotAbsent
)

// NSConstraint defines an exported type.
type NSConstraint struct {
	Kind      NSConstraintKind
	HasTarget bool
	HasLocal  bool
	Off       uint32
	Len       uint32
}

// WildcardRule defines an exported type.
type WildcardRule struct {
	NS       NSConstraint
	PC       ProcessContents
	TargetNS NamespaceID
}
