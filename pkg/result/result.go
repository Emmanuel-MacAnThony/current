package result

// Result is the explicit success-or-error return used by every flow: exactly one
// of Value / Err is meaningful. Callers check IsOk before reading Value. We use
// this instead of bare (T, error) so a flow's outcome — success or one of its
// named error states — is a single value that's easy to map at the edge.
type Result[T any] struct {
	Value T
	Err   error
}

func Ok[T any](v T) Result[T] {
	return Result[T]{Value: v}
}

func Fail[T any](err error) Result[T] {
	return Result[T]{Err: err}
}

func (r Result[T]) IsOk() bool {
	return r.Err == nil
}
