package valuew

// Coalesce returns the first non-zero value from vals.
// If all values are zero, it returns the zero value of T.
//
// Merge pattern: dst.Field = Coalesce(src.Field, dst.Field)
// Default pattern: cfg.Field = Coalesce(cfg.Field, "default")
func Coalesce[T comparable](vals ...T) T {
	var zero T
	for _, v := range vals {
		if v != zero {
			return v
		}
	}
	return zero
}