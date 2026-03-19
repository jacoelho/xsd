package valruntime

import "github.com/jacoelho/xsd/internal/runtime"

// Route groups validator kinds by the execution path they use in the validator
// runtime.
type Route uint8

const (
	RouteInvalid Route = iota
	RouteAtomic
	RouteTemporal
	RouteAnyURI
	RouteQName
	RouteHexBinary
	RouteBase64Binary
	RouteList
	RouteUnion
)

// CanonicalRoute returns the execution family for canonical validation.
func CanonicalRoute(kind runtime.ValidatorKind) Route {
	switch kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		return RouteAtomic
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		return RouteTemporal
	case runtime.VAnyURI:
		return RouteAnyURI
	case runtime.VQName, runtime.VNotation:
		return RouteQName
	case runtime.VHexBinary:
		return RouteHexBinary
	case runtime.VBase64Binary:
		return RouteBase64Binary
	case runtime.VList:
		return RouteList
	case runtime.VUnion:
		return RouteUnion
	default:
		return RouteInvalid
	}
}

// NoCanonicalRoute returns the execution family for no-canonical validation.
func NoCanonicalRoute(kind runtime.ValidatorKind) Route {
	switch kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		return RouteAtomic
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		return RouteTemporal
	case runtime.VAnyURI:
		return RouteAnyURI
	case runtime.VHexBinary:
		return RouteHexBinary
	case runtime.VBase64Binary:
		return RouteBase64Binary
	case runtime.VList:
		return RouteList
	default:
		return RouteInvalid
	}
}
