package validator

func dropStacksOverCap(limit int, stacks ...interface {
	Cap() int
	Drop()
}) {
	for _, stack := range stacks {
		if stack != nil && stack.Cap() > limit {
			stack.Drop()
		}
	}
}

func shrinkSliceCap[T any](in []T, limit int) []T {
	if cap(in) > limit {
		return nil
	}
	return in
}

func shrinkNormStack(stack [][]byte, byteLimit, entryLimit int) [][]byte {
	if len(stack) == 0 {
		return stack
	}
	for i, buf := range stack {
		if cap(buf) > byteLimit {
			stack[i] = nil
		} else {
			stack[i] = buf[:0]
		}
	}
	if len(stack) > entryLimit {
		return nil
	}
	return stack
}
