package subprocess

type SubprocessError struct {
	Id        string
	Err       error
	UserError bool
}

func (e *SubprocessError) Error() string {
	return e.Id + ": " + e.Err.Error()
}

func NewSubprocessError(user bool, id string, err error) *SubprocessError {
	return &SubprocessError{Id: id, Err: err, UserError: user}
}

func IsUserError(err error) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*SubprocessError)
	if ok {
		return e.UserError
	}
	return false
}
