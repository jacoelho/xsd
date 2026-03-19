package start

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

// Result describes the resolved element/type and xsi:nil handling for one start event.
type Result struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Nilled bool
	Skip   bool
}

// NameInput carries the start-element name bytes plus whether they are already cached.
type NameInput struct {
	Local  []byte
	NS     []byte
	Cached bool
}

// FramePlan describes the pure frame fields derived from the runtime type.
type FramePlan struct {
	Local   []byte
	NS      []byte
	Model   runtime.ModelRef
	Content runtime.ContentKind
	Mixed   bool
}

// PlanFrame derives content/model metadata and stable name bytes for one start frame.
func PlanFrame(name NameInput, result Result, typ runtime.Type, complexTypes []runtime.ComplexType) (FramePlan, error) {
	plan := FramePlan{}
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
			return FramePlan{}, fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := complexTypes[typ.Complex.ID]
		plan.Content = ct.Content
		plan.Mixed = ct.Mixed
		plan.Model = ct.Model
	default:
		return FramePlan{}, fmt.Errorf("unknown type kind %d", typ.Kind)
	}

	return plan, nil
}
