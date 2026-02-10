package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *compiler) validateEnumSets(lexical, normalized string, typ model.Type, ctx map[string]string) error {
	validatorID, err := c.compileType(typ)
	if err != nil {
		return err
	}
	if validatorID == 0 {
		return nil
	}
	enumIDs := c.enumIDsForValidator(validatorID)
	if len(enumIDs) == 0 {
		return nil
	}
	keys, err := c.keyBytesForNormalized(lexical, normalized, typ, ctx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("value not in enumeration")
	}
	table := c.enums.table()
	for _, key := range keys {
		matched := true
		for _, enumID := range enumIDs {
			if !runtime.EnumContains(&table, enumID, key.kind, key.bytes) {
				matched = false
				break
			}
		}
		if matched {
			return nil
		}
	}
	return fmt.Errorf("value not in enumeration")
}

func (c *compiler) enumIDsForValidator(id runtime.ValidatorID) []runtime.EnumID {
	if id == 0 {
		return nil
	}
	if int(id) >= len(c.bundle.Meta) {
		return nil
	}
	meta := c.bundle.Meta[id]
	if meta.Facets.Len == 0 {
		return nil
	}
	start := meta.Facets.Off
	var out []runtime.EnumID
	for i := uint32(0); i < meta.Facets.Len; i++ {
		instr := c.facets[start+i]
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	return out
}
