package service

type ServiceError struct {
	Id  string
	Err error
}

func (e *ServiceError) Error() string {
	return e.Id + ": " + e.Err.Error()
}

func NewServiceError(id string, err error) *ServiceError {
	//if err == nil {
	//	return nil
	//}

	e, ok := err.(*ServiceError)
	if ok {
		return &ServiceError{Id: id + "/" + e.Id, Err: e.Err}
	}

	return &ServiceError{Id: id, Err: err}
}
