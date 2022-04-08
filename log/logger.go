package log

type Logger interface {
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Panic(msg string, fields ...interface{})

	With(fields ...interface{}) Logger
}
