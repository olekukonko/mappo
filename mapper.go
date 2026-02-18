// mapper.go
package mappo

import (
	"fmt"
	"sort"
)

// KeyValuePair represents a single key-value pair.
type KeyValuePair[K comparable, V any] struct {
	Key   K
	Value V
}

// Mapper is a generic map type with comparable keys and any value type.
// It provides type-safe operations on maps with additional convenience methods.
type Mapper[K comparable, V any] map[K]V

// NewMapper creates and returns a new initialized Mapper.
func NewMapper[K comparable, V any]() Mapper[K, V] {
	return make(Mapper[K, V])
}

// NewMapperFrom creates a Mapper from existing map.
func NewMapperFrom[K comparable, V any](m map[K]V) Mapper[K, V] {
	if m == nil {
		return nil
	}
	return Mapper[K, V](m)
}

// Get returns the value associated with the key.
// If the key doesn't exist, returns the zero value.
func (m Mapper[K, V]) Get(key K) V {
	if m == nil {
		var zero V
		return zero
	}
	return m[key]
}

// OK returns the value and a boolean indicating whether the key exists.
func (m Mapper[K, V]) OK(key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	val, ok := m[key]
	return val, ok
}

// Set sets the value for the specified key.
func (m Mapper[K, V]) Set(key K, value V) Mapper[K, V] {
	if m != nil {
		m[key] = value
	}
	return m
}

// Delete removes the specified key.
func (m Mapper[K, V]) Delete(key K) Mapper[K, V] {
	if m != nil {
		delete(m, key)
	}
	return m
}

// Has returns true if the key exists.
func (m Mapper[K, V]) Has(key K) bool {
	if m == nil {
		return false
	}
	_, exists := m[key]
	return exists
}

// Len returns the number of elements.
func (m Mapper[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m)
}

// Keys returns a slice containing all keys.
func (m Mapper[K, V]) Keys() []K {
	if m == nil || len(m) == 0 {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Clear removes all elements.
func (m Mapper[K, V]) Clear() {
	if m == nil {
		return
	}
	for k := range m {
		delete(m, k)
	}
}

// Values returns a slice containing all values.
func (m Mapper[K, V]) Values() []V {
	if m == nil || len(m) == 0 {
		return nil
	}
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// ForEach iterates over each key-value pair.
func (m Mapper[K, V]) ForEach(fn func(K, V)) {
	for k, v := range m {
		fn(k, v)
	}
}

// Filter returns a new Mapper containing only pairs that satisfy the predicate.
func (m Mapper[K, V]) Filter(fn func(K, V) bool) Mapper[K, V] {
	result := NewMapper[K, V]()
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}

// MapValues returns a new Mapper with transformed values.
func (m Mapper[K, V]) MapValues(fn func(V) V) Mapper[K, V] {
	result := NewMapper[K, V]()
	for k, v := range m {
		result[k] = fn(v)
	}
	return result
}

// Clone returns a shallow copy.
func (m Mapper[K, V]) Clone() Mapper[K, V] {
	if m == nil {
		return nil
	}
	result := NewMapper[K, V]()
	for k, v := range m {
		result[k] = v
	}
	return result
}

// ToSlice converts to a slice of key-value pairs.
func (m Mapper[K, V]) ToSlice() []KeyValuePair[K, V] {
	if m == nil || len(m) == 0 {
		return nil
	}
	result := make([]KeyValuePair[K, V], 0, len(m))
	for k, v := range m {
		result = append(result, KeyValuePair[K, V]{Key: k, Value: v})
	}
	return result
}

// SortedKeys returns keys sorted by natural order (if possible).
func (m Mapper[K, V]) SortedKeys() []K {
	keys := m.Keys()
	if len(keys) == 0 {
		return keys
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := any(keys[i]), any(keys[j])

		switch va := a.(type) {
		case int:
			if vb, ok := b.(int); ok {
				return va < vb
			}
		case int8:
			if vb, ok := b.(int8); ok {
				return va < vb
			}
		case int16:
			if vb, ok := b.(int16); ok {
				return va < vb
			}
		case int32:
			if vb, ok := b.(int32); ok {
				return va < vb
			}
		case int64:
			if vb, ok := b.(int64); ok {
				return va < vb
			}
		case uint:
			if vb, ok := b.(uint); ok {
				return va < vb
			}
		case uint8:
			if vb, ok := b.(uint8); ok {
				return va < vb
			}
		case uint16:
			if vb, ok := b.(uint16); ok {
				return va < vb
			}
		case uint32:
			if vb, ok := b.(uint32); ok {
				return va < vb
			}
		case uint64:
			if vb, ok := b.(uint64); ok {
				return va < vb
			}
		case float32:
			if vb, ok := b.(float32); ok {
				return va < vb
			}
		case float64:
			if vb, ok := b.(float64); ok {
				return va < vb
			}
		case string:
			if vb, ok := b.(string); ok {
				return va < vb
			}
		case bool:
			if vb, ok := b.(bool); ok {
				return !va && vb // false < true
			}
		default:
			// Fallback to string comparison
			return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
		}
		return false
	})

	return keys
}

// NewBoolMapper creates a Mapper[K, bool] with keys set to true.
func NewBoolMapper[K comparable](keys ...K) Mapper[K, bool] {
	if len(keys) == 0 {
		return nil
	}
	mapper := NewMapper[K, bool]()
	for _, key := range keys {
		mapper.Set(key, true)
	}
	return mapper
}

// NewIntMapper creates a Mapper[K, int] with keys set to 0.
func NewIntMapper[K comparable](keys ...K) Mapper[K, int] {
	if len(keys) == 0 {
		return nil
	}
	mapper := NewMapper[K, int]()
	for _, key := range keys {
		mapper.Set(key, 0)
	}
	return mapper
}

// NewIdentityMapper creates a Mapper[K, K] where key == value.
func NewIdentityMapper[K comparable](keys ...K) Mapper[K, K] {
	if len(keys) == 0 {
		return nil
	}
	mapper := NewMapper[K, K]()
	for _, key := range keys {
		mapper.Set(key, key)
	}
	return mapper
}

// Merge combines multiple mappers (later ones override earlier ones).
func Merge[K comparable, V any](maps ...Mapper[K, V]) Mapper[K, V] {
	result := NewMapper[K, V]()
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
