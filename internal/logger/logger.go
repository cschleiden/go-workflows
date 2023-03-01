package logger

import (
	"fmt"
	"log"

	lg "github.com/cschleiden/go-workflows/log"
	"github.com/fatih/color"
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

	result = append(result, color.GreenString("|%s|", level))
	result = append(result, color.New(color.Bold, color.FgWhite).Sprintf("%-30s", msg))

	for i := 0; i < len(dl.defaultFields)/2; i++ {
		name := color.New(color.FgHiBlue).Sprintf("%v", dl.defaultFields[i*2])
		value := color.New(color.Faint).Sprintf("%v", dl.defaultFields[i*2+1])
		result = append(result, fmt.Sprintf("%v=%v", name, value))
	}

	for i := 0; i < len(fields)/2; i++ {
		name := color.New(color.FgHiBlue).Sprintf("%v", fields[i*2])
		value := color.New(color.Faint).Sprintf("%v", fields[i*2+1])
		result = append(result, fmt.Sprintf("%v=%v", name, value))
	}

	return result
}
