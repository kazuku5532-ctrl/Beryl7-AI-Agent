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

func resolveConstValue(expr ast.Expr, constMap map[string]string) string {
	if expr == nil {
		return ""
	}
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.ToLower(strings.Trim(lit.Value, `"`+"`"+`'`))
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if val, exists := constMap[ident.Name]; exists {
			return strings.ToLower(strings.Trim(val, `"`+"`"+`'`))
		}
	}
	return ""
}

func isHeaderGetCall(fset *token.FileSet, call *ast.CallExpr, constMap map[string]string, headerVars map[string]bool, headerFieldNames map[string]bool) bool {
	if call == nil {
		return false
	}
	sel, okSel := call.Fun.(*ast.SelectorExpr)
	if !okSel || sel.Sel.Name != "Get" {
		return false
	}

	// Verify argument evaluates to "origin" via constMap or direct string literal
	if len(call.Args) == 1 {
		argVal := resolveConstValue(call.Args[0], constMap)
		if argVal == "origin" {
			receiverRaw := strings.ToLower(renderNode(fset, sel.X))
			// [Fix 4] Resolve *ast.Ident receiver (e.g. h.Get, hdr.Get)
			if ident, okId := sel.X.(*ast.Ident); okId {
				if headerVars[ident.Name] || headerFieldNames[ident.Name] || strings.Contains(receiverRaw, "header") || strings.HasSuffix(receiverRaw, ".h") || strings.Contains(receiverRaw, "hdr") || strings.Contains(receiverRaw, "head") {
					return true
				}
			}
			// [Fix 4] Resolve *ast.SelectorExpr receiver (e.g. req.Hdr.Get, w.Request.Header.Get)
			// Catches custom structs where the Header field name does NOT contain "header"
			if selExpr, okSE := sel.X.(*ast.SelectorExpr); okSE {
				if headerVars[selExpr.Sel.Name] || headerFieldNames[selExpr.Sel.Name] || strings.Contains(receiverRaw, "header") || strings.Contains(receiverRaw, "hdr") || strings.Contains(receiverRaw, "head") {
					return true
				}
			}
			if strings.Contains(receiverRaw, "hdr") || strings.Contains(receiverRaw, "head") || strings.Contains(receiverRaw, "header") || headerFieldNames[sel.Sel.Name] {
				return true
			}
			// Universal fallback for any .Get("Origin") call on a selector or struct field
			return true
		}
	}
	return false
}

func isHeaderIndexExpr(fset *token.FileSet, indexExpr *ast.IndexExpr, constMap map[string]string, headerVars map[string]bool, headerFieldNames map[string]bool) bool {
	if indexExpr == nil {
		return false
	}

	// Resolve index key expression via constMap or direct string literal
	idxVal := resolveConstValue(indexExpr.Index, constMap)
	if idxVal == "origin" {
		receiverRaw := strings.ToLower(renderNode(fset, indexExpr.X))
		// [Fix 5] Resolve *ast.Ident receiver (e.g. h["Origin"], hdr["Origin"])
		if ident, okId := indexExpr.X.(*ast.Ident); okId {
			if headerVars[ident.Name] || headerFieldNames[ident.Name] || strings.Contains(receiverRaw, "header") || strings.HasSuffix(receiverRaw, ".h") || strings.Contains(receiverRaw, "hdr") || strings.Contains(receiverRaw, "head") {
				return true
			}
		}
		// [Fix 5] Resolve *ast.SelectorExpr receiver (e.g. req.Hdr["Origin"], ctx.Request.Header["Origin"])
		// Catches custom structs where the Header field is accessed via a selector chain
		if selExpr, okSE := indexExpr.X.(*ast.SelectorExpr); okSE {
			if headerVars[selExpr.Sel.Name] || headerFieldNames[selExpr.Sel.Name] || strings.Contains(receiverRaw, "header") || strings.Contains(receiverRaw, "hdr") || strings.Contains(receiverRaw, "head") {
				return true
			}
		}
		if strings.Contains(receiverRaw, "hdr") || strings.Contains(receiverRaw, "head") || strings.Contains(receiverRaw, "header") {
			return true
		}
		// Universal fallback for any ["Origin"] map indexing on a struct field or selector
		return true
	}
	return false
}

func isHeaderContainerExpr(fset *token.FileSet, expr ast.Expr, headerVars map[string]bool, headerFieldNames map[string]bool) bool {
	if expr == nil {
		return false
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return headerVars[ident.Name] || headerFieldNames[ident.Name]
	}
	raw := strings.ToLower(strings.TrimSpace(renderNode(fset, expr)))
	return strings.HasSuffix(raw, ".header") || strings.HasSuffix(raw, ".header()") || strings.Contains(raw, "header") || strings.HasSuffix(raw, ".hdr") || strings.HasSuffix(raw, ".head")
}

func isExprTainted(fset *token.FileSet, expr ast.Expr, taintedVars map[string]bool, constMap map[string]string, headerVars map[string]bool, headerFieldNames map[string]bool) bool {
	if expr == nil {
		return false
	}

	// 1. Direct Identifier check against exact tainted variable set or exact "origin" / "Origin" name
	if ident, ok := expr.(*ast.Ident); ok {
		lowerName := strings.ToLower(ident.Name)
		return taintedVars[ident.Name] || lowerName == "origin"
	}

	// 2. Direct Header.Get("Origin") Call Expression check with receiver & constMap validation
	if call, ok := expr.(*ast.CallExpr); ok {
		if isHeaderGetCall(fset, call, constMap, headerVars, headerFieldNames) {
			return true
		}
	}

	// 3. Direct Header["Origin"] Map Index Access check (e.g., r.Header["Origin"] or req.Hdr["Origin"])
	if indexExpr, okIdx := expr.(*ast.IndexExpr); okIdx {
		if isHeaderIndexExpr(fset, indexExpr, constMap, headerVars, headerFieldNames) {
			return true
		}
	}

	// 4. Clean Selector Expression check (e.g., req.Header.Get("origin") or r.Header.Get("origin"))
	raw := strings.ToLower(strings.TrimSpace(renderNode(fset, expr)))
	if raw == "origin" || strings.HasSuffix(raw, ".origin") || strings.Contains(raw, `header.get("origin")`) {
		return true
	}

	// 5. Recursive Sub-Node Inspection: Catches wrapped calls like strings.ToLower(req.Hdr["Origin"])
	var foundTaint bool
	ast.Inspect(expr, func(n ast.Node) bool {
		if n == nil || foundTaint {
			return false
		}
		// Sub-Node Check A: Tainted identifier usage inside nested expression
		if ident, ok := n.(*ast.Ident); ok {
			lowerName := strings.ToLower(ident.Name)
			if taintedVars[ident.Name] || lowerName == "origin" {
				foundTaint = true
				return false
			}
		}
		// Sub-Node Check B: Header.Get("Origin") call nested inside wrapper functions
		if call, ok := n.(*ast.CallExpr); ok {
			if isHeaderGetCall(fset, call, constMap, headerVars, headerFieldNames) {
				foundTaint = true
				return false
			}
		}
		// Sub-Node Check C: Header["Origin"] map index nested inside wrapper functions
		if indexExpr, okIdx := n.(*ast.IndexExpr); okIdx {
			if isHeaderIndexExpr(fset, indexExpr, constMap, headerVars, headerFieldNames) {
				foundTaint = true
				return false
			}
		}
		return true
	})

	return foundTaint
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

		// PER-FILE SCOPE ISOLATION: Re-initialize maps per file
		constMap := make(map[string]string)
		headerFieldNames := make(map[string]bool)

		// First Pass: Resolve Struct Field Names of type Header and local Constants with Compiler-Exact Repetition
		ast.Inspect(node, func(n ast.Node) bool {
			// 1. Analyze Struct Definitions to discover fields representing HTTP Headers (e.g. Hdr http.Header)
			if structType, okStruct := n.(*ast.StructType); okStruct {
				for _, field := range structType.Fields.List {
					fieldTypeRaw := strings.ToLower(renderNode(fset, field.Type))
					if strings.Contains(fieldTypeRaw, "header") || strings.Contains(fieldTypeRaw, "map[string]") {
						for _, fieldName := range field.Names {
							headerFieldNames[fieldName.Name] = true
						}
					}
				}
			}

			// 2. Resolve Constants with Compiler-Exact Multi-Variable & Type-Safe Implicit Repetition
			if genDecl, okGen := n.(*ast.GenDecl); okGen && genDecl.Tok == token.CONST {
				var lastValues []ast.Expr
				for _, spec := range genDecl.Specs {
					if valSpec, okVal := spec.(*ast.ValueSpec); okVal {
						if len(valSpec.Values) > 0 {
							// [Fix 6] Validate that the current row has at least one resolvable string value.
							// If ALL values in this row are non-string (e.g. someNumber = 123), reset lastValues
							// to nil so they do NOT propagate as implicit repeated values for the next row.
							// Failing to do this causes False Positives: a later `otherVar` with no explicit
							// value would inherit "origin" from a previous string const row.
							hasAnyStringVal := false
							for _, v := range valSpec.Values {
								if resolveConstValue(v, constMap) != "" {
									hasAnyStringVal = true
									break
								}
							}
							if hasAnyStringVal {
								lastValues = valSpec.Values
							} else {
								// Non-string row (e.g. iota, numeric literal) — break the implicit chain
								lastValues = nil
							}
						}
						for i, name := range valSpec.Names {
							var valExpr ast.Expr
							if i < len(valSpec.Values) {
								valExpr = valSpec.Values[i]
							} else if i < len(lastValues) {
								valExpr = lastValues[i]
							}
							if valExpr != nil {
								valStr := resolveConstValue(valExpr, constMap)
								if valStr != "" {
									constMap[name.Name] = valStr
								}
							}
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
			headerVars := make(map[string]bool)
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
					if isHeaderContainerExpr(fset, stmt.rhs, headerVars, headerFieldNames) && !headerVars[stmt.lhsName] {
						headerVars[stmt.lhsName] = true
						changed = true
					}
					if !taintedVars[stmt.lhsName] && isExprTainted(fset, stmt.rhs, taintedVars, constMap, headerVars, headerFieldNames) {
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
							if isExprTainted(fset, arg0, taintedVars, constMap, headerVars, headerFieldNames) {
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
