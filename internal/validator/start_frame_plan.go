package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

// StartResult describes the resolved element/type and xsi:nil handling for one start event.
type StartResult struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Nilled bool
	Skip   bool
}

// StartNameInput carries the start-element name bytes plus whether they are already cached.
type StartNameInput struct {
	Local  []byte
	NS     []byte
	Cached bool
}

// StartFramePlan describes the pure frame fields derived from the runtime type.
type StartFramePlan struct {
	Local   []byte
	NS      []byte
	Model   runtime.ModelRef
	Content runtime.ContentKind
	Mixed   bool
}

// PlanStartFrame derives content/model metadata and stable name bytes for one start frame.
func PlanStartFrame(name StartNameInput, result StartResult, typ runtime.Type, complexTypes []runtime.ComplexType) (StartFramePlan, error) {
	plan := StartFramePlan{}
	if !name.Cached {
		if len(name.Local) > 0 {
			plan.Local = slices.Clone(name.Local)
		}
		if len(name.NS) > 0 {
			plan.NS = slices.Clone(name.NS)
		}
	}

	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		plan.Content = runtime.ContentSimple
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(complexTypes) {
			return StartFramePlan{}, fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := complexTypes[typ.Complex.ID]
		plan.Content = ct.Content
		plan.Mixed = ct.Mixed
		plan.Model = ct.Model
	default:
		return StartFramePlan{}, fmt.Errorf("unknown type kind %d", typ.Kind)
	}

	return plan, nil
}
