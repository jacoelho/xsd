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

// startNameInput carries the start-element name bytes plus whether they are already cached.
type startNameInput struct {
	Local  []byte
	NS     []byte
	Cached bool
}

// startFramePlan describes the pure frame fields derived from the runtime type.
type startFramePlan struct {
	Local   []byte
	NS      []byte
	Model   runtime.ModelRef
	Content runtime.ContentKind
	Mixed   bool
}

// planStartFrame derives content/model metadata and stable name bytes for one start frame.
func planStartFrame(name startNameInput, result StartResult, typ runtime.Type, complexTypes []runtime.ComplexType) (startFramePlan, error) {
	plan := startFramePlan{}
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
			return startFramePlan{}, fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := complexTypes[typ.Complex.ID]
		plan.Content = ct.Content
		plan.Mixed = ct.Mixed
		plan.Model = ct.Model
	default:
		return startFramePlan{}, fmt.Errorf("unknown type kind %d", typ.Kind)
	}

	return plan, nil
}
