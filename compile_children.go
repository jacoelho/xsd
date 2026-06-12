package xsd

import "slices"

// childRule classifies one kind of child element inside a schema component
// and the errors its misplacement produces.
type childRule struct {
	match func(local string) bool
	// forbiddenMsg, when set, rejects the child outright.
	forbiddenMsg string
	orderMsg     string
	dupMsg       string
	// level orders sections; a child whose level is lower than one already
	// seen is out of order. Rules sharing a level are unordered relative to
	// each other.
	level int
	// maxOne rejects a second child matched by this rule.
	maxOne bool
	// terminal rejects any further children after this one.
	terminal bool
}

// childOrder describes the permitted children of one schema component:
// optional leading annotations followed by ordered sections of rules.
type childOrder struct {
	invalidMsg         func(local string) string
	annotationFirstMsg string
	rules              []childRule
	singleAnnotation   bool
}

func checkOrderedChildren(n *rawNode, order childOrder) error {
	var seen uint16
	annotationSeen := false
	nonAnnotationSeen := false
	terminalSeen := false
	maxLevelSeen := -1
	for child := range n.xsdChildren() {
		if terminalSeen {
			return schemaCompileAt(child, ErrSchemaContentModel, order.invalidMsg(child.Name.Local))
		}
		if child.Name.Local == xsdElemAnnotation {
			if nonAnnotationSeen || (order.singleAnnotation && annotationSeen) {
				return schemaCompileAt(child, ErrSchemaContentModel, order.annotationFirstMsg)
			}
			annotationSeen = true
			continue
		}
		idx := -1
		for i, rule := range order.rules {
			if rule.match(child.Name.Local) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return schemaCompileAt(child, ErrSchemaContentModel, order.invalidMsg(child.Name.Local))
		}
		rule := order.rules[idx]
		if rule.forbiddenMsg != "" {
			return schemaCompileAt(child, ErrSchemaContentModel, rule.forbiddenMsg)
		}
		nonAnnotationSeen = true
		if maxLevelSeen > rule.level {
			return schemaCompileAt(child, ErrSchemaContentModel, rule.orderMsg)
		}
		if seen&(1<<idx) != 0 && rule.maxOne {
			return schemaCompileAt(child, ErrSchemaContentModel, rule.dupMsg)
		}
		seen |= 1 << idx
		maxLevelSeen = max(maxLevelSeen, rule.level)
		terminalSeen = rule.terminal
	}
	return nil
}

func matchLocal(locals ...string) func(string) bool {
	return func(local string) bool {
		return slices.Contains(locals, local)
	}
}
