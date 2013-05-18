package tools

import "strings"

type componentError struct {
	error
	components []string
}

func NewComponentError(err error, c ...string) error {
	if err != nil {
		if e, ok := err.(*componentError); ok {
			prev := e.components
			if len(prev) > 0 && prev[0] == c[len(c) - 1] {
				prev = prev[1:]
			}
			return &componentError{
				error: e.error,
				components: append(c, prev...),
			}
		}
		return &componentError{
			error: err,
			components: c,
		}
	}
	return err
}

func (e *componentError) Error() string {
	return strings.Join(e.components, "/") + ": " + e.error.Error()
}

func GetErrorComponents(err error) []string {
	if err != nil {
		if e, ok := err.(*componentError); ok {
			return e.components
		}
	}
	return nil
}

type ErrorContext struct {
	Context string
}

func NewContext(s string) *ErrorContext {
	return &ErrorContext{Context: s,}
}

func (c *ErrorContext) NewError(err error, s ...string) error {
	return NewComponentError(err, append([]string{c.Context,}, s...)...)
}
