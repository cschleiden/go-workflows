package tester

import (
	"fmt"
	"strings"

	"github.com/cschleiden/go-workflows/internal/logger"
	"github.com/cschleiden/go-workflows/log"
)

type debugLogger struct {
	defaultFields []interface{}
	lines         *[]string
	l             log.Logger
}

func newDebugLogger() *debugLogger {
	lines := []string{}
	return &debugLogger{
		lines:         &lines,
		defaultFields: []interface{}{},
		l:             logger.NewDefaultLogger(),
	}
}

func (dl *debugLogger) hasLine(msg string) bool {
	for _, line := range *dl.lines {
		if strings.Contains(line, msg) {
			return true
		}
	}

	return false
}

func (dl *debugLogger) formatFields(level, msg string, fields ...interface{}) string {
	var result []string

	result = append(result, fmt.Sprintf("|%s| %s", level, msg))

	for i := 0; i < len(dl.defaultFields)/2; i++ {
		result = append(result, fmt.Sprintf("%v=%v", dl.defaultFields[i*2], dl.defaultFields[i*2+1]))
	}

	for i := 0; i < len(fields)/2; i++ {
		result = append(result, fmt.Sprintf("%v=%v", fields[i*2], fields[i*2+1]))
	}

	return strings.Join(result, " ")
}

func (dl *debugLogger) addLine(level, msg string, fields ...interface{}) {
	// Persist for debugging
	*dl.lines = append(*dl.lines, dl.formatFields(level, msg, fields...))
}

func (dl *debugLogger) Debug(msg string, fields ...interface{}) {
	dl.addLine("DEBUG", msg, fields...)
	dl.l.Debug(msg, fields...)
}

func (dl *debugLogger) Error(msg string, fields ...interface{}) {
	dl.addLine("ERROR", msg, fields...)
	dl.l.Error(msg, fields...)
}

func (dl *debugLogger) Panic(msg string, fields ...interface{}) {
	dl.addLine("PANIC", msg, fields...)
	dl.l.Panic(msg, fields...)
}

func (dl *debugLogger) Warn(msg string, fields ...interface{}) {
	dl.addLine("WARN", msg, fields...)
	dl.l.Warn(msg, fields...)
}

func (dl *debugLogger) With(fields ...interface{}) log.Logger {
	return &debugLogger{
		lines:         dl.lines, // Keep this here
		defaultFields: append(dl.defaultFields, fields...),
		l:             dl.l.With(fields...),
	}
}

var _ log.Logger = (*debugLogger)(nil)
