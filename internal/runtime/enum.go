package runtime

// EnumContains checks if a typed key is a member of the enumeration identified by enumID.
// The key must match both the ValueKind and the canonical bytes.
func EnumContains(table *EnumTable, enumID EnumID, kind ValueKind, key []byte) bool {
	if table == nil {
		return false
	}
	if enumID == 0 || int(enumID) >= len(table.Off) {
		return false
	}
	if table.Len[enumID] == 0 {
		return false
	}
	hashLen := table.HashLen[enumID]
	hashOff := table.HashOff[enumID]
	if hashLen == 0 {
		return false
	}
	if int(hashOff+hashLen) > len(table.Hashes) || int(hashOff+hashLen) > len(table.Slots) {
		return false
	}
	hash := HashKey(kind, key)
	mask := uint64(hashLen - 1)
	slot := int(hash & mask)
	off := table.Off[enumID]
	for i := 0; i < int(hashLen); i++ {
		idx := int(hashOff) + slot
		slotVal := table.Slots[idx]
		if slotVal == 0 {
			return false
		}
		if table.Hashes[idx] == hash {
			valueIndex := slotVal - 1
			if valueIndex < table.Len[enumID] {
				keyIndex := off + valueIndex
				if int(keyIndex) >= len(table.Keys) {
					return false
				}
				entry := table.Keys[keyIndex]
				if entry.Kind != kind {
					goto next
				}
				if bytesEqual(entry.Bytes, key) {
					return true
				}
			}
		}
	next:
		slot = (slot + 1) & int(mask)
	}
	return false
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
