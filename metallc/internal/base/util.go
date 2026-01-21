package base

// Force a cast. This is to mark our intent to cast a value and live with the
// panic that might occur if the cast fails.
func Cast[T any](val any) T {
	if val == nil {
		var zero T
		return zero
	}
	return val.(T) //nolint:forcetypeassert // This is the point.
}
