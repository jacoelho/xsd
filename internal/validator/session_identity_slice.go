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
