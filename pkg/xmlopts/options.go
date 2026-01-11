package xmlopts

import "reflect"

// Options holds a collection of configuration values for xmltext decoders.
// It is immutable after construction.
type Options struct {
	values []any
}

// New wraps a typed option value for use with JoinOptions and GetOption.
// Callers should prefer package-specific option constructors when available.
func New[T any](value T) Options {
	return Options{values: []any{value}}
}

// JoinOptions combines multiple option sets into one in declaration order.
// Later options with the same constructor take precedence at lookup time.
func JoinOptions(srcs ...Options) Options {
	if len(srcs) == 0 {
		return Options{}
	}
	total := 0
	for _, src := range srcs {
		total += len(src.values)
	}
	if total == 0 {
		return Options{}
	}
	values := make([]any, 0, total)
	for _, src := range srcs {
		values = append(values, src.values...)
	}
	return Options{values: values}
}

// GetOption retrieves the last option value associated with the constructor.
// It returns the zero value and false when no matching option is present.
func GetOption[T any](opts Options, constructor func(T) Options) (T, bool) {
	var zero T
	sample := constructor(zero)
	if len(sample.values) == 0 {
		return zero, false
	}
	target := reflect.TypeOf(sample.values[0])
	if target == nil {
		return zero, false
	}
	want := reflect.TypeOf((*T)(nil)).Elem()

	for i := len(opts.values) - 1; i >= 0; i-- {
		value := opts.values[i]
		if reflect.TypeOf(value) != target {
			continue
		}
		rv := reflect.ValueOf(value)
		if rv.Type().AssignableTo(want) {
			return rv.Interface().(T), true
		}
		if rv.Type().ConvertibleTo(want) {
			return rv.Convert(want).Interface().(T), true
		}
		return zero, false
	}
	return zero, false
}
