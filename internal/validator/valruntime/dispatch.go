package valruntime

import "github.com/jacoelho/xsd/internal/runtime"

// CanonicalCallbacks supplies the route-specific canonicalization operations.
type CanonicalCallbacks[T any] struct {
	Atomic       func() (T, error)
	Temporal     func() (T, error)
	AnyURI       func() (T, error)
	QName        func() (T, error)
	HexBinary    func() (T, error)
	Base64Binary func() (T, error)
	List         func() (T, error)
	Union        func() (T, error)
	Invalid      func(runtime.ValidatorKind) error
}

// DispatchCanonical routes canonical value validation to the correct family.
func DispatchCanonical[T any](kind runtime.ValidatorKind, callbacks CanonicalCallbacks[T]) (T, error) {
	switch CanonicalRoute(kind) {
	case RouteAtomic:
		return callbacks.Atomic()
	case RouteTemporal:
		return callbacks.Temporal()
	case RouteAnyURI:
		return callbacks.AnyURI()
	case RouteQName:
		return callbacks.QName()
	case RouteHexBinary:
		return callbacks.HexBinary()
	case RouteBase64Binary:
		return callbacks.Base64Binary()
	case RouteList:
		return callbacks.List()
	case RouteUnion:
		return callbacks.Union()
	default:
		var zero T
		if callbacks.Invalid == nil {
			return zero, nil
		}
		return zero, callbacks.Invalid(kind)
	}
}

// NoCanonicalCallbacks supplies the route-specific no-canonical validation operations.
type NoCanonicalCallbacks[T any] struct {
	Atomic       func() error
	Temporal     func() error
	AnyURI       func() error
	HexBinary    func() error
	Base64Binary func() error
	List         func() error
	Result       func() T
	Invalid      func(runtime.ValidatorKind) error
}

// DispatchNoCanonical routes no-canonical validation to the correct family.
func DispatchNoCanonical[T any](kind runtime.ValidatorKind, callbacks NoCanonicalCallbacks[T]) (T, error) {
	var zero T

	var err error
	switch NoCanonicalRoute(kind) {
	case RouteAtomic:
		err = callbacks.Atomic()
	case RouteTemporal:
		err = callbacks.Temporal()
	case RouteAnyURI:
		err = callbacks.AnyURI()
	case RouteHexBinary:
		err = callbacks.HexBinary()
	case RouteBase64Binary:
		err = callbacks.Base64Binary()
	case RouteList:
		err = callbacks.List()
	default:
		if callbacks.Invalid != nil {
			err = callbacks.Invalid(kind)
		}
	}
	if err != nil {
		return zero, err
	}
	if callbacks.Result == nil {
		return zero, nil
	}
	return callbacks.Result(), nil
}
