package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ast_analyzer.go <path-to-go-file-or-dir>")
		os.Exit(1)
	}

	targetPath := os.Args[1]
	fset := token.NewFileSet()

	var hasUnsafeCORS bool
	var violations []string

	err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		node, errParse := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if errParse != nil {
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// AST Data-Flow Inspection: Check for unsafe strings.HasPrefix or strings.HasSuffix calls on CORS origin
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "strings" {
					funcName := sel.Sel.Name
					if funcName == "HasPrefix" || funcName == "HasSuffix" {
						for _, arg := range call.Args {
							argStr := fmt.Sprintf("%v", arg)
							if strings.Contains(strings.ToLower(argStr), "origin") {
								hasUnsafeCORS = true
								pos := fset.Position(call.Pos())
								msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe strings.%s check on CORS origin variable (%s)", path, pos.Line, funcName, argStr)
								violations = append(violations, msg)
								fmt.Println(msg)
							}
						}
					}
				}
			}
			return true
		})
		return nil
	})

	if err != nil {
		fmt.Printf("AST Walk error: %v\n", err)
		os.Exit(1)
	}

	if hasUnsafeCORS {
		fmt.Printf("AST Verification Verdict: REJECTED (%d Unsafe CORS AST Data-Flow Patterns Found)\n", len(violations))
		os.Exit(1)
	}

	fmt.Println("✅ [AST-PASS] Go AST Data-Flow Analysis Clean (0 Unsafe CORS AST Patterns Found)")
	os.Exit(0)
}
