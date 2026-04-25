package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (c *artifactCompiler) valueKeysForNormalized(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]runtime.ValueKey, error) {
	keys, err := c.irValueKeysForNormalized(lexical, normalized, spec, ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.ValueKey, 0, len(keys))
	for _, key := range keys {
		kind, err := runtimeValueKind(key.Kind)
		if err != nil {
			return nil, err
		}
		out = append(out, c.makeValueKey(kind, key.Bytes))
	}
	return out, nil
}

func (c *artifactCompiler) irValueKeysForNormalized(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]schemair.ValueKey, error) {
	return schemair.ValueKeysForNormalized(lexical, normalized, spec, ctx, c.specForRef)
}

func (c *artifactCompiler) makeValueKey(kind runtime.ValueKind, bytes []byte) runtime.ValueKey {
	copied := append([]byte(nil), bytes...)
	return runtime.ValueKey{
		Kind:  kind,
		Bytes: copied,
		Hash:  runtime.HashKey(kind, copied),
	}
}

func runtimeValueKind(kind schemair.ValueKeyKind) (runtime.ValueKind, error) {
	switch kind {
	case schemair.ValueKeyBool:
		return runtime.VKBool, nil
	case schemair.ValueKeyDecimal:
		return runtime.VKDecimal, nil
	case schemair.ValueKeyFloat32:
		return runtime.VKFloat32, nil
	case schemair.ValueKeyFloat64:
		return runtime.VKFloat64, nil
	case schemair.ValueKeyString:
		return runtime.VKString, nil
	case schemair.ValueKeyBinary:
		return runtime.VKBinary, nil
	case schemair.ValueKeyQName:
		return runtime.VKQName, nil
	case schemair.ValueKeyDateTime:
		return runtime.VKDateTime, nil
	case schemair.ValueKeyDuration:
		return runtime.VKDuration, nil
	case schemair.ValueKeyList:
		return runtime.VKList, nil
	default:
		return runtime.VKInvalid, fmt.Errorf("unsupported value key kind %d", kind)
	}
}
