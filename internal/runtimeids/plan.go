package runtimeids

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

// Plan stores deterministic runtime ID assignments derived from a schema registry.
type Plan struct {
	BuiltinTypeNames []model.TypeName
	BuiltinTypeIDs   map[model.TypeName]runtime.TypeID
	TypeIDs          map[schema.TypeID]runtime.TypeID
	ElementIDs       map[schema.ElemID]runtime.ElemID
	AttributeIDs     map[schema.AttrID]runtime.AttrID
}

// Build constructs a deterministic runtime ID assignment plan.
func Build(registry *schema.Registry) (*Plan, error) {
	if registry == nil {
		return nil, fmt.Errorf("runtime ids: registry is nil")
	}
	builtin := builtinTypeNames()
	plan := &Plan{
		BuiltinTypeNames: append([]model.TypeName(nil), builtin...),
		BuiltinTypeIDs:   make(map[model.TypeName]runtime.TypeID, len(builtin)),
		TypeIDs:          make(map[schema.TypeID]runtime.TypeID, len(registry.TypeOrder)),
		ElementIDs:       make(map[schema.ElemID]runtime.ElemID, len(registry.ElementOrder)),
		AttributeIDs:     make(map[schema.AttrID]runtime.AttrID, len(registry.AttributeOrder)),
	}

	nextType := runtime.TypeID(1)
	for _, name := range builtin {
		plan.BuiltinTypeIDs[name] = nextType
		nextType++
	}
	for _, entry := range registry.TypeOrder {
		plan.TypeIDs[entry.ID] = nextType
		nextType++
	}

	nextElem := runtime.ElemID(1)
	for _, entry := range registry.ElementOrder {
		plan.ElementIDs[entry.ID] = nextElem
		nextElem++
	}

	nextAttr := runtime.AttrID(1)
	for _, entry := range registry.AttributeOrder {
		plan.AttributeIDs[entry.ID] = nextAttr
		nextAttr++
	}

	return plan, nil
}

// BuiltinTypeNames returns the deterministic builtin type runtime order.
func BuiltinTypeNames() []model.TypeName {
	builtin := builtinTypeNames()
	out := make([]model.TypeName, len(builtin))
	copy(out, builtin)
	return out
}

func builtinTypeNames() []model.TypeName {
	return []model.TypeName{
		builtins.TypeNameAnyType,
		builtins.TypeNameAnySimpleType,
		builtins.TypeNameString,
		builtins.TypeNameBoolean,
		builtins.TypeNameDecimal,
		builtins.TypeNameFloat,
		builtins.TypeNameDouble,
		builtins.TypeNameDuration,
		builtins.TypeNameDateTime,
		builtins.TypeNameTime,
		builtins.TypeNameDate,
		builtins.TypeNameGYearMonth,
		builtins.TypeNameGYear,
		builtins.TypeNameGMonthDay,
		builtins.TypeNameGDay,
		builtins.TypeNameGMonth,
		builtins.TypeNameHexBinary,
		builtins.TypeNameBase64Binary,
		builtins.TypeNameAnyURI,
		builtins.TypeNameQName,
		builtins.TypeNameNOTATION,
		builtins.TypeNameNormalizedString,
		builtins.TypeNameToken,
		builtins.TypeNameLanguage,
		builtins.TypeNameName,
		builtins.TypeNameNCName,
		builtins.TypeNameID,
		builtins.TypeNameIDREF,
		builtins.TypeNameIDREFS,
		builtins.TypeNameENTITY,
		builtins.TypeNameENTITIES,
		builtins.TypeNameNMTOKEN,
		builtins.TypeNameNMTOKENS,
		builtins.TypeNameInteger,
		builtins.TypeNameLong,
		builtins.TypeNameInt,
		builtins.TypeNameShort,
		builtins.TypeNameByte,
		builtins.TypeNameNonNegativeInteger,
		builtins.TypeNamePositiveInteger,
		builtins.TypeNameUnsignedLong,
		builtins.TypeNameUnsignedInt,
		builtins.TypeNameUnsignedShort,
		builtins.TypeNameUnsignedByte,
		builtins.TypeNameNonPositiveInteger,
		builtins.TypeNameNegativeInteger,
	}
}
