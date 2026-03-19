package start

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/value"
)

// EventInput carries the pure inputs needed to resolve one start element.
type EventInput struct {
	Attrs  attrs.Classification
	NS     []byte
	Parent ChildInput
	Sym    runtime.SymbolID
	NSID   runtime.NamespaceID
	Root   bool
}

// EventResult reports the matched particle, resolved runtime result, and
// whether a child-content error should be marked on the parent frame.
type EventResult struct {
	Match              model.Match
	Result             Result
	ChildErrorReported bool
}

// ResolveEvent combines root/child start matching with xsi/type result resolution.
func ResolveEvent(rt *runtime.Schema, in EventInput, resolver value.NSResolver, step StepModelFunc) (EventResult, error) {
	var match model.Match
	childErrorReported := false

	if in.Root {
		decision, err := ResolveRoot(rt, in.Sym, in.NSID)
		if err != nil {
			return EventResult{}, err
		}
		if decision.Skip {
			return EventResult{Result: Result{Skip: true}}, nil
		}
		match = decision.Match
	} else {
		child, err := ResolveChild(in.Parent, in.Sym, in.NSID, in.NS, step)
		if err != nil {
			return EventResult{ChildErrorReported: child.ChildErrorReported}, err
		}
		match = child.Match
		childErrorReported = child.ChildErrorReported
	}

	result, err := ResolveResult(rt, match, in.Sym, in.NSID, in.NS, in.Attrs, resolver)
	if err != nil {
		return EventResult{ChildErrorReported: childErrorReported}, err
	}
	return EventResult{
		Match:              match,
		Result:             result,
		ChildErrorReported: childErrorReported,
	}, nil
}
