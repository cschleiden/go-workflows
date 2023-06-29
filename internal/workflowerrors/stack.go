package workflowerrors

import goerrors "github.com/go-errors/errors"

func stack(err error) string {
	goerr := goerrors.New(err)
	return string(goerr.Stack())
}
