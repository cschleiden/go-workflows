//go:build analyzerplugin
// +build analyzerplugin

// Custom plugin for golangci-lint
package main

import (
	"github.com/cschleiden/go-workflows/analyzer"
	"golang.org/x/tools/go/analysis"
)

func New(conf any) ([]*analysis.Analyzer, error) {
	// The configuration type will be map[string]any or []interface, it depends
	// on your configuration.
	return []*analysis.Analyzer{analyzer.New()}, nil
}
