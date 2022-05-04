package log

// Logger is a basic logger interface. Fields have to be passed in pairs as "key", "value"
type Logger interface {
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Panic(msg string, fields ...interface{})

	// With returns a logger instance that adds the given fields to every logged message
	With(fields ...interface{}) Logger
}
