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

// --- Pointer Helpers ---

// Ptr returns a pointer to v. Useful for creating pointer literals
// without a helper variable: Ptr(42) instead of v := 42; &v.
func Ptr[T any](v T) *T {
	return &v
}

// Deref safely dereferences a pointer, returning the fallback value if p is nil.
// If no fallback is provided, returns the zero value of T.
func Deref[T any](p *T, fallback ...T) T {
	if p != nil {
		return *p
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	var zero T
	return zero
}

// DerefZero safely dereferences a pointer, returning the zero value of T if p is nil.
func DerefZero[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// IsNil reports whether p is nil.
func IsNil[T any](p *T) bool {
	return p == nil
}

// EqualPtr reports whether two pointer values are equal.
// nil pointers are considered equal to each other.
// A nil pointer is never equal to a non-nil pointer.
func EqualPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// --- Conversion Helpers ---

// ToAny converts v to the any (interface{}) type.
// Useful for converting []T to []any in variadic calls.
func ToAny[T any](v T) any {
	return v
}

// FromAny type-asserts v to T, returning fallback if the assertion fails.
// If no fallback is provided, returns the zero value of T.
func FromAny[T any](v any, fallback ...T) T {
	if t, ok := v.(T); ok {
		return t
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	var zero T
	return zero
}

// --- Slice/Map Helpers ---

// CoalesceSlice returns the first non-nil, non-empty slice from the given slices.
// If all slices are nil or empty, returns nil.
func CoalesceSlice[T any](slices ...[]T) []T {
	for _, s := range slices {
		if len(s) > 0 {
			return s
		}
	}
	return nil
}

// Contains reports whether v is present in s.
func Contains[S ~[]E, E comparable](s S, v E) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}

// MapKeys returns the keys of m as a slice.
// The order is unspecified.
func MapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// MapValues returns the values of m as a slice.
// The order is unspecified.
func MapValues[M ~map[K]V, K comparable, V any](m M) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}