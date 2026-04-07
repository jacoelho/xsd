package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// StartEventInput carries the pure inputs needed to resolve one start element.
type StartEventInput struct {
	Attrs  Classification
	NS     []byte
	Parent StartChildInput
	Sym    runtime.SymbolID
	NSID   runtime.NamespaceID
	Root   bool
}

// StartEventResult reports the matched particle, resolved runtime result, and
// whether a child-content error should be marked on the parent frame.
type StartEventResult struct {
	Match              StartMatch
	Result             StartResult
	ChildErrorReported bool
}

// ResolveStartEvent combines root/child start matching with xsi/type result resolution.
func ResolveStartEvent(rt *runtime.Schema, in StartEventInput, resolver value.NSResolver, step StartStepModelFunc) (StartEventResult, error) {
	var match StartMatch
	childErrorReported := false

	if in.Root {
		decision, err := ResolveStartRoot(rt, in.Sym, in.NSID)
		if err != nil {
			return StartEventResult{}, err
		}
		if decision.Skip {
			return StartEventResult{Result: StartResult{Skip: true}}, nil
		}
		match = decision.Match
	} else {
		child, err := ResolveStartChild(in.Parent, in.Sym, in.NSID, in.NS, step)
		if err != nil {
			return StartEventResult{ChildErrorReported: child.ChildErrorReported}, err
		}
		match = child.Match
		childErrorReported = child.ChildErrorReported
	}

	result, err := ResolveStartResult(rt, match, in.Sym, in.NSID, in.NS, in.Attrs, resolver)
	if err != nil {
		return StartEventResult{ChildErrorReported: childErrorReported}, err
	}
	return StartEventResult{
		Match:              match,
		Result:             result,
		ChildErrorReported: childErrorReported,
	}, nil
}
