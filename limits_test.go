package mnema

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodebaseLimits(t *testing.T) {
	root := repoRoot(t)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		checkFileLimits(t, root, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// checkFileLimits checks one Go file: 400 lines (tests 800), functions 80.
func checkFileLimits(t *testing.T, root, path string) {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", rel, err)
		return
	}

	lineCount := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lineCount++
	}

	isTest := strings.HasSuffix(path, "_test.go")
	maxLines := 400
	if isTest {
		maxLines = 800
	}
	if lineCount > maxLines {
		t.Errorf("%s has %d lines (max %d)", rel, lineCount, maxLines)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	if err != nil {
		t.Errorf("parse %s: %v", rel, err)
		return
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		span := end - start + 1
		if span > 80 {
			t.Errorf("%s:%d func %s has %d lines (max 80)", rel, start, fn.Name.Name, span)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod not found)")
		}
		dir = parent
	}
}
