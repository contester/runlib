package subprocess

type SubprocessError struct {
	Id string
	Err error
}

func (e *SubprocessError) Error() string {
	return e.Id + ": " + e.Err.Error()
}

func NewSubprocessError(id string, err error) *SubprocessError {
	return &SubprocessError{Id: id, Err: err}
}
