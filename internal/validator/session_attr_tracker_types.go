package validator

import "github.com/jacoelho/xsd/internal/validator/attrs"

// AttributeTracker owns reusable buffers used during attribute validation.
type AttributeTracker struct {
	attrAppliedBuf []attrs.Applied
	attrState      attrs.Tracker
}
