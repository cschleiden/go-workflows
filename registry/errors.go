package registry

type ErrInvalidWorkflow struct {
	msg string
}

func (e *ErrInvalidWorkflow) Error() string {
	return e.msg
}

type ErrWorkflowAlreadyRegistered struct {
	msg string
}

func (e *ErrWorkflowAlreadyRegistered) Error() string {
	return e.msg
}

type ErrInvalidActivity struct {
	msg string
}

func (e *ErrInvalidActivity) Error() string {
	return e.msg
}

type ErrActivityAlreadyRegistered struct {
	msg string
}

func (e *ErrActivityAlreadyRegistered) Error() string {
	return e.msg
}
