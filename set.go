// set.go
package mappo

// Set is a generic set type based on Mapper.
type Set[T comparable] struct {
	m Mapper[T, struct{}]
}

// NewSet creates a new Set.
func NewSet[T comparable](elems ...T) *Set[T] {
	s := &Set[T]{m: NewMapper[T, struct{}]()}
	for _, elem := range elems {
		s.Add(elem)
	}
	return s
}

// Add adds an element to the set.
func (s *Set[T]) Add(elem T) {
	s.m.Set(elem, struct{}{})
}

// Remove removes an element from the set.
func (s *Set[T]) Remove(elem T) {
	s.m.Delete(elem)
}

// Has returns true if the element exists.
func (s *Set[T]) Has(elem T) bool {
	return s.m.Has(elem)
}

// Len returns the number of elements.
func (s *Set[T]) Len() int {
	return s.m.Len()
}

// Clear removes all elements.
func (s *Set[T]) Clear() {
	s.m.Clear()
}

// Elements returns all elements as a slice.
func (s *Set[T]) Elements() []T {
	return s.m.Keys()
}

// ForEach iterates over all elements.
func (s *Set[T]) ForEach(fn func(T)) {
	s.m.ForEach(func(k T, _ struct{}) {
		fn(k)
	})
}

// Union returns a new set with elements from both sets.
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	s.ForEach(func(elem T) {
		result.Add(elem)
	})
	other.ForEach(func(elem T) {
		result.Add(elem)
	})
	return result
}

// Intersection returns a new set with elements common to both sets.
func (s *Set[T]) Intersection(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	// Iterate over smaller set for efficiency
	if s.Len() < other.Len() {
		s.ForEach(func(elem T) {
			if other.Has(elem) {
				result.Add(elem)
			}
		})
	} else {
		other.ForEach(func(elem T) {
			if s.Has(elem) {
				result.Add(elem)
			}
		})
	}
	return result
}

// Difference returns a new set with elements in s but not in other.
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	s.ForEach(func(elem T) {
		if !other.Has(elem) {
			result.Add(elem)
		}
	})
	return result
}
