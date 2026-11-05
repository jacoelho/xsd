package validate

// RecoveryLimitReached reports whether collecting count errors satisfies the
// caller's maximum-error limit. limit <= 0 means unlimited.
func RecoveryLimitReached(count, limit int) bool {
	return limit > 0 && count >= limit
}
