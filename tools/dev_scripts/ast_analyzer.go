package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func renderNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

func isOriginExpression(exprStr string) bool {
	lower := strings.ToLower(exprStr)
	if lower == "origin" || strings.HasSuffix(lower, ".origin") || lower == `r.header.get("origin")` || lower == `r.header.get("origin")` {
		return true
	}
	return false
}

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

			// Inspect Selector Expressions (e.g. strings.HasPrefix or strings.HasSuffix)
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "strings" {
					funcName := sel.Sel.Name

					if funcName == "HasPrefix" && len(call.Args) >= 2 {
						arg0Code := renderNode(fset, call.Args[0])
						if isOriginExpression(arg0Code) {
							hasUnsafeCORS = true
							pos := fset.Position(call.Pos())
							msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe strings.HasPrefix check on CORS origin expression: '%s'", path, pos.Line, arg0Code)
							violations = append(violations, msg)
							fmt.Println(msg)
						}
					}

					if funcName == "HasSuffix" && len(call.Args) >= 2 {
						arg0Code := renderNode(fset, call.Args[0])
						arg1Code := renderNode(fset, call.Args[1])

						// Restored mDNS Suffix Inspection (.local, .lan, .home)
						if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							val := strings.ToLower(lit.Value)
							if strings.Contains(val, ".local") || strings.Contains(val, ".lan") || strings.Contains(val, ".home") {
								hasUnsafeCORS = true
								pos := fset.Position(call.Pos())
								msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe mDNS strings.HasSuffix check on target '%s' with suffix '%s'", path, pos.Line, arg0Code, arg1Code)
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
