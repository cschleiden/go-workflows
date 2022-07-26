package logger

import (
	"fmt"
	"log"

	lg "github.com/ticctech/go-workflows/log"
)

type defaultLogger struct {
	defaultFields []interface{}
}

var _ lg.Logger = (*defaultLogger)(nil)

func NewDefaultLogger() lg.Logger {
	return &defaultLogger{}
}

func (dl *defaultLogger) Debug(msg string, fields ...interface{}) {
	log.Println(dl.formatFields("DEBUG", msg, fields...)...)
}

func (dl *defaultLogger) Warn(msg string, fields ...interface{}) {
	log.Println(dl.formatFields("WARN", msg, fields...)...)
}

func (dl *defaultLogger) Error(msg string, fields ...interface{}) {
	log.Println(dl.formatFields("ERROR", msg, fields...)...)
}

func (dl *defaultLogger) Panic(msg string, fields ...interface{}) {
	log.Panicln(dl.formatFields("PANIC", msg, fields...)...)
}

func (dl *defaultLogger) With(fields ...interface{}) lg.Logger {
	return &defaultLogger{
		defaultFields: append(dl.defaultFields, fields...),
	}
}

func (dl *defaultLogger) formatFields(level, msg string, fields ...interface{}) []any {
	var result []any

	result = append(result, level)
	result = append(result, msg)

	for i := 0; i < len(dl.defaultFields)/2; i++ {
		result = append(result, fmt.Sprintf("%v=%v", dl.defaultFields[i*2], dl.defaultFields[i*2+1]))
	}

	for i := 0; i < len(fields)/2; i++ {
		result = append(result, fmt.Sprintf("%v=%v", fields[i*2], fields[i*2+1]))
	}

	return result
}
