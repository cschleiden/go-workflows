package main

import "github.com/cschleiden/go-workflows/log"

type nullLogger struct {
	defaultFields []interface{}
}

// Debug implements log.Logger
func (*nullLogger) Debug(msg string, fields ...interface{}) {
}

// Error implements log.Logger
func (*nullLogger) Error(msg string, fields ...interface{}) {
}

// Panic implements log.Logger
func (*nullLogger) Panic(msg string, fields ...interface{}) {
}

// Warn implements log.Logger
func (*nullLogger) Warn(msg string, fields ...interface{}) {
}

// With implements log.Logger
func (nl *nullLogger) With(fields ...interface{}) log.Logger {
	return nl
}

var _ log.Logger = (*nullLogger)(nil)
