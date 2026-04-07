package validator

// AttrResult holds validated input attributes and applied default/fixed attributes.
type AttrResult struct {
	Applied []Applied
	Attrs   []Start
}
