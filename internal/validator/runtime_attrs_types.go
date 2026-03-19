package validator

import "github.com/jacoelho/xsd/internal/validator/attrs"

// AttrResult holds validated input attributes and applied default/fixed attributes.
type AttrResult struct {
	Applied []attrs.Applied
	Attrs   []attrs.Start
}
