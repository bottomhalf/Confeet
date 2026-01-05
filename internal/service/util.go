package service

func Filter[T any](items []T, fn func(T) bool) []T {
	var result []T
	for _, v := range items {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

func WhereSlice[T any](items []T, fn func(T) bool) []T {
	if len(items) == 0 {
		return nil
	}

	// Pre-allocate with estimated capacity to reduce allocations
	result := make([]T, 0, len(items)/2)

	for _, v := range items {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

func WhereMap[R comparable, T any](items map[R]T, fn func(R, T) bool) []T {
	if len(items) == 0 {
		return nil
	}

	// Pre-allocate with estimated capacity to reduce allocations
	result := make([]T, 0, len(items)/2)

	for r, v := range items {
		if fn(r, v) {
			result = append(result, v)
		}
	}
	return result
}

func FilterMap[K comparable, V any](m map[K]V, fn func(K, V) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}
