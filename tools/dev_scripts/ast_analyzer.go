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
	"regexp"
	"strings"
)

func renderNodeNormalized(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	raw := buf.String()
	re := regexp.MustCompile(`\s*([(),.])\s*`)
	return strings.ToLower(re.ReplaceAllString(raw, "$1"))
}

func isOriginExpression(exprStr string) bool {
	clean := strings.TrimSpace(exprStr)
	return clean == "origin" ||
		strings.HasSuffix(clean, ".origin") ||
		clean == `r.header.get("origin")` ||
		clean == `req.header.get("origin")`
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

	constMap := make(map[string]string)

	err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		node, errParse := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if errParse != nil {
			return nil
		}

		// First pass: resolve constant string declarations
		ast.Inspect(node, func(n ast.Node) bool {
			if spec, ok := n.(*ast.ValueSpec); ok {
				for i, name := range spec.Names {
					if i < len(spec.Values) {
						if lit, okLit := spec.Values[i].(*ast.BasicLit); okLit && lit.Kind == token.STRING {
							constMap[name.Name] = strings.ToLower(lit.Value)
						}
					}
				}
			}
			return true
		})

		// Second pass: inspect AST calls
		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "strings" {
					funcName := sel.Sel.Name

					if (funcName == "HasPrefix" || funcName == "HasSuffix") && len(call.Args) >= 2 {
						arg0Code := renderNodeNormalized(fset, call.Args[0])

						// Scoped check: Only evaluate if target expression represents an Origin
						if isOriginExpression(arg0Code) {
							pos := fset.Position(call.Pos())

							if funcName == "HasPrefix" {
								hasUnsafeCORS = true
								msg := fmt.Sprintf("❌ [AST-FAIL] %s:%d: Unsafe strings.HasPrefix check on CORS origin expression: '%s'", path, pos.Line, arg0Code)
								violations = append(violations, msg)
								fmt.Println(msg)
							}

							if funcName == "HasSuffix" {
								suffixVal := ""
								if lit, okLit := call.Args[1].(*ast.BasicLit); okLit && lit.Kind == token.STRING {
									suffixVal = strings.ToLower(lit.Value)
								} else if ident, okIdent := call.Args[1].(*ast.Ident); okIdent {
									suffixVal = constMap[ident.Name]
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
