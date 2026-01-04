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

func FilterMap[K comparable, V any](m map[K]V, fn func(K, V) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}
