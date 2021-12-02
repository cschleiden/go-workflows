package workflow

type Context interface {
	IsReplaying() bool

	RegisterResult()
}

func NewContext() Context {
	return &contextImpl{}
}

type contextImpl struct {
}

func (c *contextImpl) IsReplaying() bool {
	return false // TODO
}
