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

func isOriginExpr(fset *token.FileSet, expr ast.Expr, taintedVars map[string]bool) bool {
	if expr == nil {
		return false
	}
	raw := strings.ToLower(strings.TrimSpace(renderNode(fset, expr)))
	
	// Check direct identifier or selector matching
	if strings.Contains(raw, "origin") {
		return true
	}
	
	// Check if identifier is in the function local tainted variable map
	if ident, ok := expr.(*ast.Ident); ok {
		if taintedVars[ident.Name] {
			return true
		}
	}
	return false
}

func resolveConstValue(expr ast.Expr, constMap map[string]string) string {
	if expr == nil {
		return ""
	}
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.ToLower(lit.Value)
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if val, exists := constMap[ident.Name]; exists {
			return val
		}
	}
	return ""
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

		// PER-FILE SCOPE ISOLATION: Re-initialize constMap per file to prevent cross-file pollution
		constMap := make(map[string]string)

		// First Pass: Resolve local constants and group constant declarations
		ast.Inspect(node, func(n ast.Node) bool {
			if spec, ok := n.(*ast.ValueSpec); ok {
				for i, name := range spec.Names {
					if i < len(spec.Values) {
						valStr := resolveConstValue(spec.Values[i], constMap)
						if valStr != "" {
							constMap[name.Name] = valStr
						}
					}
				}
			}
			return true
		})

		// Second Pass: Inspect function declarations and AST call expressions with intra-procedural taint tracking
		ast.Inspect(node, func(n ast.Node) bool {
			fn, okFn := n.(*ast.FuncDecl)
			if !okFn {
				return true
			}

			// Local Taint Map per function declaration
			taintedVars := make(map[string]bool)

			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				// Track variable assignments (e.g., originHeader := r.Header.Get("Origin"))
				if assign, okAssign := inner.(*ast.AssignStmt); okAssign {
					for i, rhs := range assign.Rhs {
						rhsCode := strings.ToLower(renderNode(fset, rhs))
						if strings.Contains(rhsCode, "origin") {
							if i < len(assign.Lhs) {
								if ident, okId := assign.Lhs[i].(*ast.Ident); okId {
									taintedVars[ident.Name] = true
								}
							}
						}
					}
				}

				// Inspect strings.HasPrefix and strings.HasSuffix calls
				call, okCall := inner.(*ast.CallExpr)
				if !okCall {
					return true
				}

				if sel, okSel := call.Fun.(*ast.SelectorExpr); okSel {
					if pkg, okPkg := sel.X.(*ast.Ident); okPkg && pkg.Name == "strings" {
						funcName := sel.Sel.Name

						if (funcName == "HasPrefix" || funcName == "HasSuffix") && len(call.Args) >= 2 {
							arg0 := call.Args[0]

							// Scoped check: Evaluate only if arg0 represents a tainted or origin expression
							if isOriginExpr(fset, arg0, taintedVars) {
								pos := fset.Position(call.Pos())
								arg0Code := renderNode(fset, arg0)

								if funcName == "HasPrefix" {
									hasUnsafeCORS = true
									msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe strings.HasPrefix check on CORS origin expression: '%s'", path, pos.Line, arg0Code)
									violations = append(violations, msg)
									fmt.Println(msg)
								}

								if funcName == "HasSuffix" {
									suffixVal := resolveConstValue(call.Args[1], constMap)
									if suffixVal == "" {
										suffixVal = strings.ToLower(renderNode(fset, call.Args[1]))
									}

									if strings.Contains(suffixVal, ".local") || strings.Contains(suffixVal, ".lan") || strings.Contains(suffixVal, ".home") {
										hasUnsafeCORS = true
										msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe mDNS strings.HasSuffix check on CORS origin '%s' with suffix '%s'", path, pos.Line, arg0Code, suffixVal)
										violations = append(violations, msg)
										fmt.Println(msg)
									}
								}
							}
						}
					}
				}
				return true
			})
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
