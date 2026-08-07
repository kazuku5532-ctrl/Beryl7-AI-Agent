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
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

func isExprTainted(fset *token.FileSet, expr ast.Expr, taintedVars map[string]bool) bool {
	if expr == nil {
		return false
	}

	// 1. Direct Identifier check against exact tainted variable set or exact "origin" / "Origin" name
	if ident, ok := expr.(*ast.Ident); ok {
		lowerName := strings.ToLower(ident.Name)
		if taintedVars[ident.Name] || lowerName == "origin" {
			return true
		}
		return false
	}

	// 2. Native AST Call Expression check for Header.Get("Origin") / Header.Get("origin")
	if call, ok := expr.(*ast.CallExpr); ok {
		if sel, okSel := call.Fun.(*ast.SelectorExpr); okSel && sel.Sel.Name == "Get" {
			if len(call.Args) == 1 {
				if lit, okLit := call.Args[0].(*ast.BasicLit); okLit && lit.Kind == token.STRING {
					cleanVal := strings.ToLower(strings.Trim(lit.Value, `"`+"`"+`'`))
					if cleanVal == "origin" {
						return true
					}
				}
			}
		}
	}

	// 3. Clean Selector Expression check (e.g., req.Header.Get("origin") or r.Header.Get("origin"))
	raw := strings.ToLower(strings.TrimSpace(renderNode(fset, expr)))
	if raw == "origin" || strings.HasSuffix(raw, ".origin") || strings.Contains(raw, `header.get("origin")`) {
		return true
	}

	// 4. Inspect AST sub-nodes for EXACT tainted identifiers (never substrings like "originalPath")
	var foundTaint bool
	ast.Inspect(expr, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			lowerName := strings.ToLower(ident.Name)
			if taintedVars[ident.Name] || lowerName == "origin" {
				foundTaint = true
				return false
			}
		}
		return true
	})

	return foundTaint
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

type assignStmtPair struct {
	lhsName string
	rhs     ast.Expr
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

		// Second Pass: High-Performance Worklist Data-Flow Taint Propagation & CORS Inspection
		ast.Inspect(node, func(n ast.Node) bool {
			fn, okFn := n.(*ast.FuncDecl)
			if !okFn {
				return true
			}

			taintedVars := make(map[string]bool)
			var statements []assignStmtPair

			// 1. Single AST Pass: Collect all assignment/declaration statements into an in-memory Worklist
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				if inner == nil {
					return true
				}

				if assign, okAssign := inner.(*ast.AssignStmt); okAssign {
					for i, rhs := range assign.Rhs {
						if i < len(assign.Lhs) {
							if ident, okId := assign.Lhs[i].(*ast.Ident); okId {
								statements = append(statements, assignStmtPair{lhsName: ident.Name, rhs: rhs})
							}
						}
					}
				}

				if declStmt, okDecl := inner.(*ast.DeclStmt); okDecl {
					if genDecl, okGen := declStmt.Decl.(*ast.GenDecl); okGen && genDecl.Tok == token.VAR {
						for _, spec := range genDecl.Specs {
							if valSpec, okVal := spec.(*ast.ValueSpec); okVal {
								for i, val := range valSpec.Values {
									if i < len(valSpec.Names) {
										statements = append(statements, assignStmtPair{lhsName: valSpec.Names[i].Name, rhs: val})
									}
								}
							}
						}
					}
				}
				return true
			})

			// 2. High-Speed Worklist Propagation (Iterates over RAM slice, 0 AST re-traversals!)
			changed := true
			for changed {
				changed = false
				for _, stmt := range statements {
					if !taintedVars[stmt.lhsName] && isExprTainted(fset, stmt.rhs, taintedVars) {
						taintedVars[stmt.lhsName] = true
						changed = true
					}
				}
			}

			// Final Pass: Inspect strings.HasPrefix and strings.HasSuffix calls on tainted expressions
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				if inner == nil {
					return true
				}
				call, okCall := inner.(*ast.CallExpr)
				if !okCall {
					return true
				}

				if sel, okSel := call.Fun.(*ast.SelectorExpr); okSel {
					if pkg, okPkg := sel.X.(*ast.Ident); okPkg && pkg.Name == "strings" {
						funcName := sel.Sel.Name

						if (funcName == "HasPrefix" || funcName == "HasSuffix") && len(call.Args) >= 2 {
							arg0 := call.Args[0]

							// Scoped check: Evaluate only if arg0 represents a tainted origin expression
							if isExprTainted(fset, arg0, taintedVars) {
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
