package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestAPIKeyQueriesFilterMerchantAndMode(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	merchantPred := regexp.MustCompile(`(?i)merchant_id\s*=`)
	modePred := regexp.MustCompile(`(?i)\bmode\s*=`)
	allowedUnscoped := map[string]bool{"GetAPIKey": true}
	var checked int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				sql := strings.Trim(lit.Value, "`\"")
				if !queryNeedsTenantFilter(sql) {
					return true
				}
				checked++
				filtered := merchantPred.MatchString(sql) && modePred.MatchString(sql)
				if !filtered && !allowedUnscoped[fn.Name.Name] {
					t.Errorf("%s reads api_keys without merchant_id and mode filters:\n%s", fn.Name.Name, sql)
				}
				return true
			})
		}
	}
	if checked < 3 {
		t.Fatalf("checked %d api_keys queries, want at least the auth lookup and the two scoped reads", checked)
	}
}

func queryNeedsTenantFilter(sql string) bool {
	fields := strings.Fields(sql)
	if len(fields) == 0 || !strings.Contains(strings.ToLower(sql), "api_keys") {
		return false
	}
	switch strings.ToUpper(fields[0]) {
	case "SELECT", "UPDATE", "DELETE":
		return true
	default:
		return false
	}
}
