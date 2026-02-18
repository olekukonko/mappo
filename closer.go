package mappo

type CloseFn func()

func Closer(it *Item) (CloseFn, bool) {
	if it == nil || it.Value == nil {
		return nil, false
	}

	if c, ok := it.Value.(interface{ Close() error }); ok {
		return func() { _ = c.Close() }, true
	}

	if c, ok := it.Value.(interface{ Close() }); ok {
		return func() { c.Close() }, true
	}

	return nil, false
}

func CloserDelete(_ string, it *Item) {
	fn, ok := Closer(it)
	if !ok {
		return
	}
	fn()
}
