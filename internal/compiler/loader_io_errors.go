package compiler

import (
	"errors"
	"io/fs"
	"os"
)

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}
