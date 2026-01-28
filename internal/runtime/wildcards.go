package runtime

type ProcessContents uint8

const (
	PCStrict ProcessContents = iota
	PCLax
	PCSkip
)

type NSConstraintKind uint8

const (
	NSAny NSConstraintKind = iota
	NSOther
	NSEnumeration
	NSNotAbsent
)

type NSConstraint struct {
	Kind      NSConstraintKind
	HasTarget bool
	HasLocal  bool
	Off       uint32
	Len       uint32
}

type WildcardRule struct {
	NS       NSConstraint
	PC       ProcessContents
	TargetNS NamespaceID
}
