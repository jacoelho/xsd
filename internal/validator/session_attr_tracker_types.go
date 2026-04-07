package validator

// AttributeTracker owns reusable buffers used during attribute validation.
type AttributeTracker struct {
	attrAppliedBuf []Applied
	attrState      Tracker
}
