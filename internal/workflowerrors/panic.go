package workflowerrors

type PanicError struct {
	message    string
	stacktrace string
}

func (pe *PanicError) Error() string {
	return pe.message
}

func (pe *PanicError) Stack() string {
	return pe.stacktrace
}

func NewPanicError(msg string) *PanicError {
	return &PanicError{
		message:    msg,
		stacktrace: stack(3), // Skip new panic error and immediate caller
	}
}
