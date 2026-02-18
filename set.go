package mappo

// Set is a generic set type based on Mapper.
type Set[T comparable] struct {
	m Mapper[T, struct{}]
}

// NewSet creates a new Set.
func NewSet[T comparable](elems ...T) *Set[T] {
	s := &Set[T]{m: NewMapper[T, struct{}]()}
	for _, elem := range elems {
		s.m[elem] = struct{}{}
	}
	return s
}

// Add adds an element to the set.
func (s *Set[T]) Add(elem T) {
	if s.m == nil {
		s.m = NewMapper[T, struct{}]()
	}
	s.m[elem] = struct{}{}
}

// Remove removes an element from the set.
func (s *Set[T]) Remove(elem T) {
	if s.m == nil {
		return
	}
	delete(s.m, elem)
}

// Has returns true if the element exists.
func (s *Set[T]) Has(elem T) bool {
	if s.m == nil {
		return false
	}
	_, exists := s.m[elem]
	return exists
}

// Len returns the number of elements.
func (s *Set[T]) Len() int {
	if s.m == nil {
		return 0
	}
	return len(s.m)
}

// Clear removes all elements.
func (s *Set[T]) Clear() {
	s.m = NewMapper[T, struct{}]()
}

// IsEmpty returns true if the set has no elements.
func (s *Set[T]) IsEmpty() bool {
	return s.Len() == 0
}

// Pop removes and returns an arbitrary element.
func (s *Set[T]) Pop() (T, bool) {
	var zero T
	if s.m == nil || len(s.m) == 0 {
		return zero, false
	}

	for elem := range s.m {
		delete(s.m, elem)
		return elem, true
	}
	return zero, false
}

// Elements returns all elements as a slice.
func (s *Set[T]) Elements() []T {
	if s.m == nil {
		return nil
	}
	elems := make([]T, 0, len(s.m))
	for elem := range s.m {
		elems = append(elems, elem)
	}
	return elems
}

// ForEach iterates over all elements.
func (s *Set[T]) ForEach(fn func(T)) {
	if s.m == nil {
		return
	}
	for elem := range s.m {
		fn(elem)
	}
}

// Filter returns a new set with elements satisfying the predicate.
func (s *Set[T]) Filter(fn func(T) bool) *Set[T] {
	result := NewSet[T]()
	s.ForEach(func(elem T) {
		if fn(elem) {
			result.Add(elem)
		}
	})
	return result
}

// Clone returns a shallow copy of the set.
func (s *Set[T]) Clone() *Set[T] {
	result := NewSet[T]()
	s.ForEach(func(elem T) {
		result.Add(elem)
	})
	return result
}

// Equal returns true if two sets contain the same elements.
func (s *Set[T]) Equal(other *Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for elem := range s.m {
		if !other.Has(elem) {
			return false
		}
	}
	return true
}

// IsSubset returns true if s is a subset of other.
func (s *Set[T]) IsSubset(other *Set[T]) bool {
	if s.Len() > other.Len() {
		return false
	}
	for elem := range s.m {
		if !other.Has(elem) {
			return false
		}
	}
	return true
}

// IsSuperset returns true if s is a superset of other.
func (s *Set[T]) IsSuperset(other *Set[T]) bool {
	return other.IsSubset(s)
}

// Union returns a new set with elements from both sets.
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := NewSet[T]()

	// Add from larger set first to minimize resizes
	if s.Len() > other.Len() {
		for elem := range s.m {
			result.m[elem] = struct{}{}
		}
		for elem := range other.m {
			result.m[elem] = struct{}{}
		}
	} else {
		for elem := range other.m {
			result.m[elem] = struct{}{}
		}
		for elem := range s.m {
			result.m[elem] = struct{}{}
		}
	}

	return result
}

// Intersection returns a new set with elements common to both sets.
func (s *Set[T]) Intersection(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	// Iterate over smaller set for efficiency
	if s.Len() < other.Len() {
		for elem := range s.m {
			if other.Has(elem) {
				result.Add(elem)
			}
		}
	} else {
		for elem := range other.m {
			if s.Has(elem) {
				result.Add(elem)
			}
		}
	}
	return result
}

// Difference returns a new set with elements in s but not in other.
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	for elem := range s.m {
		if !other.Has(elem) {
			result.Add(elem)
		}
	}
	return result
}

// SymmetricDifference returns elements in exactly one of the sets.
func (s *Set[T]) SymmetricDifference(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	for elem := range s.m {
		if !other.Has(elem) {
			result.Add(elem)
		}
	}
	for elem := range other.m {
		if !s.Has(elem) {
			result.Add(elem)
		}
	}
	return result
}
