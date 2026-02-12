package preprocessor

import "fmt"

func (l *Loader) beginLocationLoad() error {
	if l == nil {
		return fmt.Errorf("no resolver configured")
	}
	if l.resolver == nil {
		return fmt.Errorf("no resolver configured")
	}
	return nil
}
