package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// SimpleTypeFinalRole identifies the schema component role that is applying a
// simple-type final derivation rule.
type SimpleTypeFinalRole uint8

const (
	// SimpleTypeFinalBaseRestriction checks a restriction base simple type.
	SimpleTypeFinalBaseRestriction SimpleTypeFinalRole = iota
	// SimpleTypeFinalListItem checks an xs:list item type.
	SimpleTypeFinalListItem
	// SimpleTypeFinalUnionMember checks an xs:union member type.
	SimpleTypeFinalUnionMember
)

// CheckSimpleRestrictionBase rejects direct restriction of xs:anySimpleType.
func CheckSimpleRestrictionBase(baseID, anySimpleType runtime.SimpleTypeID) error {
	if baseID == anySimpleType {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "simple type cannot restrict xs:anySimpleType")
	}
	return nil
}

// CheckSimpleTypeFinalAllows maps runtime simple-type final-mask rejection into
// the compile diagnostic for the schema role being derived.
func CheckSimpleTypeFinalAllows(final, derivation runtime.DerivationMask, role SimpleTypeFinalRole) error {
	if err := runtime.ValidateSimpleTypeFinalAllows(final, derivation); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, simpleTypeFinalRoleMessage(role))
	}
	return nil
}

// checkSimpleListItemType rejects list item types whose graph reaches a list.
func checkSimpleListItemType(reachesList bool) error {
	if reachesList {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "list item type cannot be a list type")
	}
	return nil
}

type simpleTypeListReachState uint8

const (
	simpleTypeListReachUnchecked simpleTypeListReachState = iota
	simpleTypeListReachChecking
	simpleTypeListReachChecked
)

// simpleTypeListReachability memoizes reachability for the compiler's
// append-only table of completed simple types.
type simpleTypeListReachability struct {
	state   []simpleTypeListReachState
	reaches []bool
	stack   []simpleTypeListReachFrame
}

type simpleTypeListReachFrame struct {
	next     int
	id       runtime.SimpleTypeID
	unstable bool
}

func (r *simpleTypeListReachability) reachesList(types []runtime.SimpleType, id runtime.SimpleTypeID) bool {
	if !runtime.ValidSimpleTypeID(id, len(types)) {
		return false
	}
	if missing := len(types) - len(r.state); missing > 0 {
		r.state = append(r.state, make([]simpleTypeListReachState, missing)...)
		r.reaches = append(r.reaches, make([]bool, missing)...)
	}
	switch r.state[id] {
	case simpleTypeListReachChecked:
		return r.reaches[id]
	case simpleTypeListReachChecking:
		return false
	case simpleTypeListReachUnchecked:
	}

	r.state[id] = simpleTypeListReachChecking
	stack := r.stack[:0]
	stack = appendSimpleTypeListReachFrame(stack, simpleTypeListReachFrame{id: id}, len(types))
	for len(stack) != 0 {
		last := len(stack) - 1
		frame := &stack[last]
		typ := types[frame.id]
		if typ.Variety == runtime.SimpleVarietyList {
			for _, active := range stack {
				r.reaches[active.id] = true
				r.state[active.id] = simpleTypeListReachChecked
			}
			r.stack = stack[:0]
			return true
		}
		if typ.Variety != runtime.SimpleVarietyUnion || frame.next == len(typ.Union) {
			unstable := frame.unstable
			if unstable {
				r.state[frame.id] = simpleTypeListReachUnchecked
			} else {
				r.state[frame.id] = simpleTypeListReachChecked
			}
			stack = stack[:last]
			if unstable && len(stack) != 0 {
				stack[len(stack)-1].unstable = true
			}
			continue
		}

		member := typ.Union[frame.next]
		frame.next++
		if !runtime.ValidSimpleTypeID(member, len(types)) {
			continue
		}
		switch r.state[member] {
		case simpleTypeListReachChecked:
			if r.reaches[member] {
				for _, active := range stack {
					r.reaches[active.id] = true
					r.state[active.id] = simpleTypeListReachChecked
				}
				r.stack = stack[:0]
				return true
			}
		case simpleTypeListReachChecking:
			frame.unstable = true
		case simpleTypeListReachUnchecked:
			r.state[member] = simpleTypeListReachChecking
			stack = appendSimpleTypeListReachFrame(stack, simpleTypeListReachFrame{id: member}, len(types))
		}
	}
	r.stack = stack[:0]
	return false
}

func appendSimpleTypeListReachFrame(
	stack []simpleTypeListReachFrame,
	frame simpleTypeListReachFrame,
	limit int,
) []simpleTypeListReachFrame {
	if len(stack) < cap(stack) {
		return append(stack, frame)
	}
	if len(stack) >= limit {
		panic("simple type list reachability stack exceeds type count")
	}
	capacity := min(limit, max(1, cap(stack)*2))
	grown := make([]simpleTypeListReachFrame, len(stack), capacity)
	copy(grown, stack)
	return append(grown, frame)
}

func simpleTypeFinalRoleMessage(role SimpleTypeFinalRole) string {
	switch role {
	case SimpleTypeFinalBaseRestriction:
		return "base simple type final blocks restriction"
	case SimpleTypeFinalListItem:
		return "item simple type final blocks list"
	case SimpleTypeFinalUnionMember:
		return "member simple type final blocks union"
	default:
		return "simple type final blocks derivation"
	}
}
