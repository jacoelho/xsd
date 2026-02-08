package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func sliceElemICs(rt *runtime.Schema, elem *runtime.Element) ([]runtime.ICID, error) {
	if elem == nil {
		return nil, fmt.Errorf("identity: element missing")
	}
	if elem.ICLen == 0 {
		return nil, nil
	}
	off := elem.ICOff
	end := off + elem.ICLen
	if int(off) > len(rt.ElemICs) || int(end) > len(rt.ElemICs) {
		return nil, fmt.Errorf("identity: elem ICs out of range")
	}
	return rt.ElemICs[off:end], nil
}

func slicePathIDs(list []runtime.PathID, off, ln uint32) ([]runtime.PathID, error) {
	if ln == 0 {
		return nil, fmt.Errorf("identity: empty path list")
	}
	end := off + ln
	if int(off) > len(list) || int(end) > len(list) {
		return nil, fmt.Errorf("identity: path list out of range")
	}
	return list[off:end], nil
}

func splitFieldPaths(ids []runtime.PathID) ([][]runtime.PathID, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("identity: field paths empty")
	}
	hasSep := slices.Contains(ids, 0)
	if !hasSep {
		return [][]runtime.PathID{append([]runtime.PathID(nil), ids...)}, nil
	}
	fields := make([][]runtime.PathID, 0, len(ids))
	cur := make([]runtime.PathID, 0, 4)
	for _, id := range ids {
		if id == 0 {
			if len(cur) == 0 {
				return nil, fmt.Errorf("identity: empty field path set")
			}
			fields = append(fields, cur)
			cur = make([]runtime.PathID, 0, 4)
			continue
		}
		cur = append(cur, id)
	}
	if len(cur) == 0 {
		return nil, fmt.Errorf("identity: trailing field separator")
	}
	fields = append(fields, cur)
	return fields, nil
}

func isSimpleContent(rt *runtime.Schema, typeID runtime.TypeID) bool {
	if typeID == 0 || int(typeID) >= len(rt.Types) {
		return false
	}
	typ := rt.Types[typeID]
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		return true
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(rt.ComplexTypes) {
			return false
		}
		ct := rt.ComplexTypes[typ.Complex.ID]
		return ct.Content == runtime.ContentSimple
	default:
		return false
	}
}

func elementValueKey(frame *rtIdentityFrame, elem *runtime.Element, in identityEndInput) (runtime.ValueKind, []byte, bool) {
	if elem == nil {
		return runtime.VKInvalid, nil, false
	}
	if frame.nilled {
		return runtime.VKInvalid, nil, false
	}
	if in.KeyKind == runtime.VKInvalid {
		return runtime.VKInvalid, nil, true
	}
	return in.KeyKind, in.KeyBytes, true
}

func elementByID(rt *runtime.Schema, id runtime.ElemID) (*runtime.Element, bool) {
	if rt == nil || id == 0 || int(id) >= len(rt.Elements) {
		return nil, false
	}
	return &rt.Elements[id], true
}
