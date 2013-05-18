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
	if err == nil {
		return nil
	}
	if e, ok := err.(*SubprocessError); ok {
		return &SubprocessError{Id: id + "/" + e.Id, Err: e.Err, UserError: user || e.UserError}
	}
	return &SubprocessError{Id: id, Err: err, UserError: user}
}

func IsUserError(err error) bool {
	if err != nil {
		if e, ok := err.(*SubprocessError); ok {
			return e.UserError
		}
	}
	return false
}
