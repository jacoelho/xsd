package preprocessor

import (
	"errors"
	"io/fs"
	"os"
)

func joinWithClose(loadErr, closeErr error) error {
	if closeErr == nil {
		return loadErr
	}
	if loadErr == nil {
		return closeErr
	}
	return errors.Join(loadErr, closeErr)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}
