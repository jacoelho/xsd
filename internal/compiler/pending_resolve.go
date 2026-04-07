package compiler

import "fmt"

// PendingResolveCallbacks supplies the caller-owned state and side effects needed to
// resolve one source's deferred directives.
type PendingResolveCallbacks[K comparable, S any, T any] struct {
	Inputs         func(K) (*Tracking[K], []Directive[K], S, error)
	Stage          func([]Directive[K]) (T, error)
	Apply          func([]Directive[K], S, T) error
	Commit         func(T) error
	MarkMerged     func(K, []Directive[K])
	ResolveTargets func([]Directive[K]) error
}

// ResolvePending handles the generic staged-apply-commit flow for one source's
// deferred directives.
func ResolvePending[K comparable, S any, T any](sourceKey K, callbacks PendingResolveCallbacks[K, S, T]) error {
	tracking, directives, source, err := callbacks.Inputs(sourceKey)
	if err != nil || len(directives) == 0 {
		return err
	}

	staged, err := callbacks.Stage(directives)
	if err != nil {
		return err
	}
	if err := callbacks.Apply(directives, source, staged); err != nil {
		return err
	}
	if err := callbacks.Commit(staged); err != nil {
		return err
	}
	if callbacks.MarkMerged != nil {
		callbacks.MarkMerged(sourceKey, directives)
	}
	if tracking != nil {
		tracking.Clear()
	}
	if callbacks.ResolveTargets != nil {
		return callbacks.ResolveTargets(directives)
	}
	return nil
}

// PendingTargetCallbacks supplies the caller-owned tracking and recursive resolution
// behavior for one set of deferred directive targets.
type PendingTargetCallbacks[K comparable] struct {
	Tracking func(K) (*Tracking[K], error)
	Resolve  func(K) error
	Label    func(K) string
}

// PendingApplyCallbacks supplies caller-owned staged-target lookup and per-kind merge behavior.
type PendingApplyCallbacks[K comparable, S any, T any, U any] struct {
	Target  func(T, K) (U, error)
	Include func(Directive[K], S, U) error
	Import  func(Directive[K], S, U) error
}

// ApplyPending looks up each staged target and dispatches deferred directives by kind.
func ApplyPending[K comparable, S any, T any, U any](directives []Directive[K], source S, staged T, callbacks PendingApplyCallbacks[K, S, T, U]) error {
	for _, directive := range directives {
		target, err := callbacks.Target(staged, directive.TargetKey)
		if err != nil {
			return err
		}
		switch directive.Kind {
		case 0:
			if callbacks.Include == nil {
				return fmt.Errorf("missing include callback")
			}
			if err := callbacks.Include(directive, source, target); err != nil {
				return err
			}
		case 1:
			if callbacks.Import == nil {
				return fmt.Errorf("missing import callback")
			}
			if err := callbacks.Import(directive, source, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown pending directive kind: %d", directive.Kind)
		}
	}
	return nil
}

// ResolvePendingTargets decrements unresolved target counts and triggers recursive
// resolution when a target becomes ready.
func ResolvePendingTargets[K comparable](directives []Directive[K], callbacks PendingTargetCallbacks[K]) error {
	for _, directive := range directives {
		tracking, err := callbacks.Tracking(directive.TargetKey)
		if err != nil {
			return err
		}
		label := ""
		if callbacks.Label != nil {
			label = callbacks.Label(directive.TargetKey)
		}
		if err := tracking.Decrement(label); err != nil {
			return err
		}
		if tracking.Count == 0 && callbacks.Resolve != nil {
			if err := callbacks.Resolve(directive.TargetKey); err != nil {
				return err
			}
		}
	}
	return nil
}
