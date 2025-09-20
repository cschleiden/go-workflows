package analyzer

import (
	"go/ast"
	"go/types"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func init() {
	register.Plugin("goworkflows", New)
}

func New(settings any) (register.LinterPlugin, error) {
	// The configuration type will be map[string]any or []interface, it depends on your configuration.
	// You can use https://github.com/go-viper/mapstructure to convert map to struct.
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, err
	}

	return &GoWorkflowsPlugin{Settings: s}, nil
}

type GoWorkflowsPlugin struct {
	Settings Settings
}

type Settings struct {
	CheckPrivateReturnValues bool `json:"checkprivatereturnvalues"`
}

func (w *GoWorkflowsPlugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		{
			Name:     "goworkflows",
			Doc:      "Checks for common errors when writing workflows",
			Run:      w.run,
			Requires: []*analysis.Analyzer{inspect.Analyzer},
		},
	}, nil
}

func (w *GoWorkflowsPlugin) GetLoadMode() string {
	// NOTE: the mode can be `register.LoadModeSyntax` or `register.LoadModeTypesInfo`.
	// - `register.LoadModeSyntax`: if the linter doesn't use types information.
	// - `register.LoadModeTypesInfo`: if the linter uses types information.

	return register.LoadModeSyntax
}

func (w *GoWorkflowsPlugin) run(pass *analysis.Pass) (interface{}, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Expect workflows to be top level functions in a file. Therefore it should be enough to just keep track if the current
	// AST node is a descendant of a workflow FuncDecl.
	inWorkflow := true

	workflowImportName := "workflow"

	inspector.Nodes([]ast.Node{
		(*ast.File)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.ImportSpec)(nil),
		(*ast.RangeStmt)(nil),
		(*ast.SelectStmt)(nil),
		(*ast.GoStmt)(nil),
		(*ast.CallExpr)(nil),
	}, func(node ast.Node, push bool) bool {
		if _, ok := node.(*ast.File); ok {
			// New file, reset state
			inWorkflow = true
		}

		if _, ok := node.(*ast.FuncDecl); ok {
			if !push {
				// Finished with the current workflow
				inWorkflow = false
				return false
			}
		} else {
			if !push {
				// Only check nodes while descending
				return false
			}

			if !inWorkflow {
				// Only check nodes while in a top-level workflow func
				return false
			}
		}

		switch n := node.(type) {
		case *ast.ImportSpec:
			if n.Path.Value == `"github.com/cschleiden/go-workflows/workflow"` {
				if n.Name != nil {
					workflowImportName = n.Name.Name
				} else {
					workflowImportName = "workflow"
				}
			}

		case *ast.FuncDecl:
			// Only check functions that look like workflows
			if !isWorkflow(workflowImportName, n) {
				return false
			}

			inWorkflow = true

			// Check return types
			if n.Name.IsExported() || w.Settings.CheckPrivateReturnValues {
				if n.Type.Results == nil || len(n.Type.Results.List) == 0 {
					pass.Reportf(n.Pos(), "workflow `%v` doesn't return anything. needs to return at least `error`", n.Name.Name)
				} else {
					if len(n.Type.Results.List) > 2 {
						pass.Reportf(n.Pos(), "workflow `%v` returns more than two values", n.Name.Name)
						return true
					}

					lastResult := n.Type.Results.List[len(n.Type.Results.List)-1]
					if types.ExprString(lastResult.Type) != "error" {
						pass.Reportf(n.Pos(), "workflow `%v` doesn't return `error` as last return value", n.Name.Name)
					}
				}
			}

			funcScope := pass.TypesInfo.Scopes[n.Type]
			if funcScope != nil {
				checkVarsInScope(pass, funcScope)
			}

			// Continue with the function's children
			return true

		case *ast.RangeStmt:
			{
				t := pass.TypesInfo.TypeOf(n.X)
				if t == nil {
					break
				}

				switch t.(type) {
				case *types.Map:
					pass.Reportf(n.Pos(), "iterating over a `map` is not deterministic and not allowed in workflows")

				case *types.Chan:
					pass.Reportf(n.Pos(), "using native channels is not allowed in workflows, use `workflow.Channel` instead")
				}
			}

		case *ast.SelectStmt:
			pass.Reportf(n.Pos(), "`select` statements are not allowed in workflows, use `workflow.Select` instead")

		case *ast.GoStmt:
			pass.Reportf(n.Pos(), "use `workflow.Go` instead of `go` in workflows")

		case *ast.CallExpr:
			var pkg *ast.Ident
			var id *ast.Ident
			switch fun := n.Fun.(type) {
			case *ast.SelectorExpr:
				pkg, _ = fun.X.(*ast.Ident)
				id = fun.Sel
			}

			if pkg == nil || id == nil {
				break
			}

			pkgInfo := pass.TypesInfo.Uses[pkg]
			pkgName, _ := pkgInfo.(*types.PkgName)
			if pkgName == nil {
				break
			}

			switch pkgName.Imported().Path() {
			case "time":
				switch id.Name {
				case "Now":
					pass.Reportf(n.Pos(), "`time.Now` is not allowed in workflows, use `workflow.Now` instead")
				case "Sleep":
					pass.Reportf(n.Pos(), "`time.Sleep` is not allowed in workflows, use `workflow.Sleep` instead")
				}
			}
		}

		// Continue with the children
		return true
	})

	return nil, nil
}

func checkVarsInScope(pass *analysis.Pass, scope *types.Scope) {
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		switch t := obj.Type().(type) {
		case *types.Chan:
			pass.Reportf(obj.Pos(), "using native channels is not allowed in workflows, use `workflow.Channel` instead")

		case *types.Named:
			checkNamed(pass, obj, t)

		case *types.Pointer:
			if named, ok := t.Elem().(*types.Named); ok {
				checkNamed(pass, obj, named)
			}
		}
	}

	for i := 0; i < scope.NumChildren(); i++ {
		checkVarsInScope(pass, scope.Child(i))
	}
}

func checkNamed(pass *analysis.Pass, ref types.Object, named *types.Named) {
	if obj := named.Obj(); obj != nil {
		if pkg := obj.Pkg(); pkg != nil {
			switch pkg.Path() {
			case "sync":
				if obj.Name() == "WaitGroup" {
					pass.Reportf(ref.Pos(), "using `sync.WaitGroup` is not allowed in workflows, use `workflow.WaitGroup` instead")
				}
			}
		}
	}

}

func isWorkflow(workflowImportName string, funcDecl *ast.FuncDecl) bool {
	params := funcDecl.Type.Params.List

	// Need at least workflow.Context
	if len(params) < 1 {
		return false
	}

	firstParam, ok := params[0].Type.(*ast.SelectorExpr)
	if !ok { // first param type isn't identificator so it can't be of type "string"
		return false
	}

	xname, ok := firstParam.X.(*ast.Ident)
	if !ok {
		return false
	}

	selname := firstParam.Sel.Name
	if xname.Name+"."+selname != workflowImportName+".Context" {
		return false
	}

	return true
}
