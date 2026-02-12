package runtime

// ProcessContents encodes strict/lax/skip wildcard processing.
type ProcessContents uint8

const (
	PCStrict ProcessContents = iota
	PCLax
	PCSkip
)

// NSConstraintKind enumerates ns constraint kind values.
type NSConstraintKind uint8

const (
	NSAny NSConstraintKind = iota
	NSOther
	NSEnumeration
	NSNotAbsent
)

// NSConstraint stores lowered namespace-matching rules for wildcards.
type NSConstraint struct {
	Kind      NSConstraintKind
	HasTarget bool
	HasLocal  bool
	Off       uint32
	Len       uint32
}

// WildcardRule stores one compiled wildcard namespace/process policy.
type WildcardRule struct {
	NS       NSConstraint
	PC       ProcessContents
	TargetNS NamespaceID
}
