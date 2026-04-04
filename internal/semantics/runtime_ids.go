package semantics

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

// RuntimeIDPlan stores deterministic runtime ID assignments derived from a schema registry.
type RuntimeIDPlan struct {
	BuiltinTypeIDs   map[model.TypeName]runtime.TypeID
	TypeIDs          map[ids.TypeID]runtime.TypeID
	ElementIDs       map[ids.ElemID]runtime.ElemID
	AttributeIDs     map[ids.AttrID]runtime.AttrID
	BuiltinTypeNames []model.TypeName
}

// BuildRuntimeIDPlan constructs a deterministic runtime ID assignment plan.
func BuildRuntimeIDPlan(registry *analysis.Registry) (*RuntimeIDPlan, error) {
	if registry == nil {
		return nil, fmt.Errorf("runtime ids: registry is nil")
	}
	builtin := builtinTypeNames()
	plan := &RuntimeIDPlan{
		BuiltinTypeNames: slices.Clone(builtin),
		BuiltinTypeIDs:   make(map[model.TypeName]runtime.TypeID, len(builtin)),
		TypeIDs:          make(map[ids.TypeID]runtime.TypeID, len(registry.TypeOrder)),
		ElementIDs:       make(map[ids.ElemID]runtime.ElemID, len(registry.ElementOrder)),
		AttributeIDs:     make(map[ids.AttrID]runtime.AttrID, len(registry.AttributeOrder)),
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
	return slices.Clone(builtin)
}

func builtinTypeNames() []model.TypeName {
	return []model.TypeName{
		model.TypeNameAnyType,
		model.TypeNameAnySimpleType,
		model.TypeNameString,
		model.TypeNameBoolean,
		model.TypeNameDecimal,
		model.TypeNameFloat,
		model.TypeNameDouble,
		model.TypeNameDuration,
		model.TypeNameDateTime,
		model.TypeNameTime,
		model.TypeNameDate,
		model.TypeNameGYearMonth,
		model.TypeNameGYear,
		model.TypeNameGMonthDay,
		model.TypeNameGDay,
		model.TypeNameGMonth,
		model.TypeNameHexBinary,
		model.TypeNameBase64Binary,
		model.TypeNameAnyURI,
		model.TypeNameQName,
		model.TypeNameNOTATION,
		model.TypeNameNormalizedString,
		model.TypeNameToken,
		model.TypeNameLanguage,
		model.TypeNameName,
		model.TypeNameNCName,
		model.TypeNameID,
		model.TypeNameIDREF,
		model.TypeNameIDREFS,
		model.TypeNameENTITY,
		model.TypeNameENTITIES,
		model.TypeNameNMTOKEN,
		model.TypeNameNMTOKENS,
		model.TypeNameInteger,
		model.TypeNameLong,
		model.TypeNameInt,
		model.TypeNameShort,
		model.TypeNameByte,
		model.TypeNameNonNegativeInteger,
		model.TypeNamePositiveInteger,
		model.TypeNameUnsignedLong,
		model.TypeNameUnsignedInt,
		model.TypeNameUnsignedShort,
		model.TypeNameUnsignedByte,
		model.TypeNameNonPositiveInteger,
		model.TypeNameNegativeInteger,
	}
}
