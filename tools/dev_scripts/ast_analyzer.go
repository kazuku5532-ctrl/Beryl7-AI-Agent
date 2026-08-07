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

// [Fix 5] Duyệt đệ quy SelectorExpr để trích xuất tên receiver chuẩn xác,
// chống trốn quét qua Struct Fields bất kể bao nhiêu tầng lồng (req.Hdr, ctx.Request.Header, v.v.)
func getRootReceiverName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return getRootReceiverName(v.X) + "." + v.Sel.Name
	default:
		return ""
	}
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

func isHeaderGetCall(fset *token.FileSet, call *ast.CallExpr, constMap map[string]string, headerVars map[string]bool) bool {
	if call == nil {
		return false
	}
	sel, okSel := call.Fun.(*ast.SelectorExpr)
	if !okSel || sel.Sel.Name != "Get" {
		return false
	}

	// [Fix 5] Sử dụng getRootReceiverName để bắt gọn cấu trúc lồng như req.Hdr.Get(...)
	receiverRaw := strings.ToLower(getRootReceiverName(sel.X))
	if !(strings.Contains(receiverRaw, "header") || strings.HasSuffix(receiverRaw, ".h") || headerVars[receiverRaw]) {
		return false
	}

	if len(call.Args) == 1 {
		// Fix OCR bug: call.Args[0] not call.Args
		argVal := resolveConstValue(call.Args[0], constMap)
		if argVal == "origin" {
			return true
		}
	}
	return false
}

func isHeaderIndexExpr(fset *token.FileSet, indexExpr *ast.IndexExpr, constMap map[string]string, headerVars map[string]bool) bool {
	if indexExpr == nil {
		return false
	}

	// [Fix 5] Sử dụng getRootReceiverName để bắt gọn req.Hdr["Origin"], ctx.Request.Header["Origin"], v.v.
	receiverRaw := strings.ToLower(getRootReceiverName(indexExpr.X))
	if !(strings.Contains(receiverRaw, "header") || strings.HasSuffix(receiverRaw, ".h") || headerVars[receiverRaw]) {
		return false
	}

	idxVal := resolveConstValue(indexExpr.Index, constMap)
	return idxVal == "origin"
}

func isHeaderContainerExpr(fset *token.FileSet, expr ast.Expr, headerVars map[string]bool) bool {
	if expr == nil {
		return false
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return headerVars[ident.Name]
	}
	raw := strings.ToLower(strings.TrimSpace(getRootReceiverName(expr)))
	return strings.HasSuffix(raw, ".header") || strings.HasSuffix(raw, ".header()") || strings.Contains(raw, "header")
}

func isExprTainted(fset *token.FileSet, expr ast.Expr, taintedVars map[string]bool, constMap map[string]string, headerVars map[string]bool) bool {
	if expr == nil {
		return false
	}

	// 1. Direct Identifier check against exact tainted variable set or "origin" name
	if ident, ok := expr.(*ast.Ident); ok {
		lowerName := strings.ToLower(ident.Name)
		return taintedVars[ident.Name] || lowerName == "origin"
	}

	// 2. Direct Header.Get("Origin") Call Expression check
	if call, ok := expr.(*ast.CallExpr); ok {
		if isHeaderGetCall(fset, call, constMap, headerVars) {
			return true
		}
	}

	// 3. Direct Header["Origin"] Map Index Access check
	if indexExpr, okIdx := expr.(*ast.IndexExpr); okIdx {
		if isHeaderIndexExpr(fset, indexExpr, constMap, headerVars) {
			return true
		}
	}

	// 4. Selector expression fallback (e.g. req.Header.Get("origin") rendered as string)
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
		if ident, ok := n.(*ast.Ident); ok {
			lowerName := strings.ToLower(ident.Name)
			if taintedVars[ident.Name] || lowerName == "origin" {
				foundTaint = true
				return false
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if isHeaderGetCall(fset, call, constMap, headerVars) {
				foundTaint = true
				return false
			}
		}
		if indexExpr, okIdx := n.(*ast.IndexExpr); okIdx {
			if isHeaderIndexExpr(fset, indexExpr, constMap, headerVars) {
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
	// Fix OCR bug: os.Args[1] not os.Args[15]
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

		// PER-FILE SCOPE: Re-initialize maps per file
		constMap := make(map[string]string)
		// [Fix 3 & Fix 4] lastRowValues []string: ánh xạ chính xác theo chỉ mục cột,
		// reset về nil khi gặp hằng số phi chuỗi (iota, số nguyên) để chặn nhiễm độc trạng thái.
		var lastRowValues []string

		// First Pass: Resolve Constants with Compiler-Exact Multi-Variable Repetition
		ast.Inspect(node, func(n ast.Node) bool {
			if genDecl, okGen := n.(*ast.GenDecl); okGen && genDecl.Tok == token.CONST {
				for _, spec := range genDecl.Specs {
					if valSpec, okVal := spec.(*ast.ValueSpec); okVal {
						if len(valSpec.Values) > 0 {
							// Build lastRowValues from current row's resolved values
							newRow := make([]string, len(valSpec.Names))
							hasAnyString := false
							for i, val := range valSpec.Values {
								valStr := resolveConstValue(val, constMap)
								if i < len(newRow) {
									newRow[i] = valStr
								}
								if valStr != "" {
									hasAnyString = true
									constMap[valSpec.Names[i].Name] = valStr
								}
							}
							if hasAnyString {
								// [Fix 3] Store full row slice so multi-variable implicit repetition maps correctly by index
								lastRowValues = newRow
							} else {
								// [Fix 4] Non-string row (iota/numeric) — break the implicit chain to prevent False Positives
								lastRowValues = nil
							}
						} else if len(lastRowValues) > 0 {
							// Implicit repetition: map by column index from previous row
							for i, name := range valSpec.Names {
								if i < len(lastRowValues) && lastRowValues[i] != "" {
									constMap[name.Name] = lastRowValues[i]
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

			// 1. Single AST Pass: Collect all assignment/declaration statements into Worklist
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

			// 2. High-Speed Worklist Propagation (iterates over RAM slice, 0 AST re-traversals)
			changed := true
			for changed {
				changed = false
				for _, stmt := range statements {
					if isHeaderContainerExpr(fset, stmt.rhs, headerVars) && !headerVars[stmt.lhsName] {
						headerVars[stmt.lhsName] = true
						changed = true
					}
					if !taintedVars[stmt.lhsName] && isExprTainted(fset, stmt.rhs, taintedVars, constMap, headerVars) {
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
							// Fix OCR bug: call.Args[0] not call.Args
							arg0 := call.Args[0]
							if isExprTainted(fset, arg0, taintedVars, constMap, headerVars) {
								pos := fset.Position(call.Pos())
								arg0Code := renderNode(fset, arg0)

								if funcName == "HasPrefix" {
									hasUnsafeCORS = true
									msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe strings.HasPrefix check on CORS origin expression: '%s'", path, pos.Line, arg0Code)
									violations = append(violations, msg)
									fmt.Println(msg)
								}

								if funcName == "HasSuffix" {
									// Fix OCR bug: call.Args[1] not call.Args[15]
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
