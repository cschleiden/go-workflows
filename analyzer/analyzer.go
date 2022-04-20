package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "goworkflows",
	Doc:      "Checks for common errors when writing workflows",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}

	inspector.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)

		if !isWorkflow(funcDecl) {
			return
		}

		// Check return types
		if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
			pass.Reportf(funcDecl.Pos(), "workflow %q doesn't return anything. needs to return at least `error`", funcDecl.Name.Name)
		} else {
			if len(funcDecl.Type.Results.List) > 2 {
				pass.Reportf(funcDecl.Pos(), "workflow %q returns more than two values", funcDecl.Name.Name)
				return
			}

			lastResult := funcDecl.Type.Results.List[len(funcDecl.Type.Results.List)-1]
			if types.ExprString(lastResult.Type) != "error" {
				pass.Reportf(funcDecl.Pos(), "workflow %q doesn't return `error` as last return value", funcDecl.Name.Name)
			}
		}

		// Check for various errors in the workflow body
		for _, stmt := range funcDecl.Body.List {
			switch stmt := stmt.(type) {
			// Check for map iterations
			case *ast.RangeStmt:
				{
					t := pass.TypesInfo.TypeOf(stmt.X)
					if t == nil {
						continue
					}

					if _, ok := t.(*types.Map); !ok {
						continue
					}

					pass.Reportf(stmt.Pos(), "iterating over a map is not deterministic and not allowed in workflows")
				}

			// Check for `go` statements
			case *ast.GoStmt:
				pass.Reportf(stmt.Pos(), "use workflow.Go instead of `go` in workflows")
			}
		}
	})

	return nil, nil
}

func isWorkflow(funcDecl *ast.FuncDecl) bool {
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
	if xname.Name+"."+selname != "workflow.Context" {
		return false
	}

	return true
}
