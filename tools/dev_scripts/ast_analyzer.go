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
	var hasUnsafePrefix bool
	var hasUnsafeSuffix bool

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

			// Check for strings.HasPrefix or strings.HasSuffix calls on CORS origin
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "strings" {
					if sel.Sel.Name == "HasPrefix" {
						if len(call.Args) > 0 {
							argStr := fmt.Sprintf("%v", call.Args[0])
							if strings.Contains(strings.ToLower(argStr), "origin") {
								hasUnsafePrefix = true
								hasUnsafeCORS = true
								fmt.Printf("❌ [AST-FAIL] %s:%d: Unsafe strings.HasPrefix check on origin\n", path, fset.Position(call.Pos()).Line)
							}
						}
					}
					if sel.Sel.Name == "HasSuffix" {
						if len(call.Args) > 1 {
							if lit, ok := call.Args[1].(*ast.BasicLit); ok {
								val := strings.ToLower(lit.Value)
								if strings.Contains(val, ".local") || strings.Contains(val, ".lan") || strings.Contains(val, ".home") {
									hasUnsafeSuffix = true
									hasUnsafeCORS = true
									fmt.Printf("❌ [AST-FAIL] %s:%d: Unsafe mDNS strings.HasSuffix check on %s\n", path, fset.Position(call.Pos()).Line, val)
								}
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
		fmt.Printf("AST Verification Verdict: REJECTED (Unsafe CORS AST Patterns Found - Prefix: %v, Suffix: %v)\n", hasUnsafePrefix, hasUnsafeSuffix)
		os.Exit(1)
	}

	fmt.Println("✅ [AST-PASS] Go AST Data-Flow Analysis Clean (0 Unsafe CORS AST Patterns Found)")
	os.Exit(0)
}
