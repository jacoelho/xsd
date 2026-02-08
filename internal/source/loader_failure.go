package source

import "fmt"

func (l *SchemaLoader) beginLocationLoad() error {
	if l == nil {
		return fmt.Errorf("no resolver configured")
	}
	if l.failed {
		return l.failedError()
	}
	if l.resolver == nil {
		err := fmt.Errorf("no resolver configured")
		l.markFailed(err)
		return err
	}
	return nil
}

func (l *SchemaLoader) markFailed(err error) {
	if l == nil || err == nil || l.failed {
		return
	}
	l.failed = true
	l.failure = err
}

func (l *SchemaLoader) failedError() error {
	if l == nil {
		return errLoaderFailed
	}
	if l.failure == nil {
		return fmt.Errorf("%w: create a new loader", errLoaderFailed)
	}
	return fmt.Errorf("%w: create a new loader (first failure: %w)", errLoaderFailed, l.failure)
}
