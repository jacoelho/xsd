package validator

func (s *Session) appendEndError(errs []error, path *string, err error) []error {
	if err == nil {
		return errs
	}
	if path != nil && *path == "" {
		*path = s.pathString()
	}
	return append(errs, err)
}

func (s *Session) ensurePath(path *string) {
	if path == nil || *path != "" {
		return
	}
	*path = s.pathString()
}
