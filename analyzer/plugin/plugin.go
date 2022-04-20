//go:build analyzerplugin
// +build analyzerplugin

// Custom plugin for golangci-lint
package main

import (
	"github.com/cschleiden/go-workflows/analyzer"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct{}

func (*analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		analyzer.Analyzer,
	}
}

var AnalyzerPlugin analyzerPlugin
