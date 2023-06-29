package workflowerrors

type PanicError struct {
	message    string
	stacktrace string
}

func (pe *PanicError) Error() string {
	return pe.message
}

func (pe *PanicError) Stacktrace() string {
	return pe.stacktrace
}

func NewPanicError(msg string) error {
	return &PanicError{
		message: msg,
	}
}
