package preprocessor

import "github.com/jacoelho/xsd/internal/parser"

// ApplyCallbacks supplies the root-owned state transitions needed to commit one
// already-parsed schema into the loader graph.
type ApplyCallbacks[E any] struct {
	Begin           func() (*E, func(), error)
	Init            func(*E) error
	ApplyDirectives func() error
	Commit          func(*E)
	ResolvePending  func() error
	RollbackPending func()
	Rollback        func(*E)
}

// ApplyParsed commits one parsed schema into the load graph.
func ApplyParsed[E any](sch *parser.Schema, callbacks ApplyCallbacks[E]) (*parser.Schema, error) {
	entry, cleanup, err := callbacks.Begin()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if callbacks.Init != nil {
		if err := callbacks.Init(entry); err != nil {
			return nil, err
		}
	}
	if callbacks.ApplyDirectives != nil {
		if err := callbacks.ApplyDirectives(); err != nil {
			return nil, err
		}
	}
	if callbacks.Commit != nil {
		callbacks.Commit(entry)
	}
	if callbacks.ResolvePending != nil {
		if err := callbacks.ResolvePending(); err != nil {
			if callbacks.RollbackPending != nil {
				callbacks.RollbackPending()
			}
			if callbacks.Rollback != nil {
				callbacks.Rollback(entry)
			}
			return nil, err
		}
	}

	return sch, nil
}
