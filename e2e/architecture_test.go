package e2e

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const modulePath = "example.com/sms-platform/"

// allowedImports is the dependency graph of ARCHITECTURE-SPINE (AD-19). Keys
// are internal packages relative to internal/; values are the internal
// packages they may import. Packages not listed here (cmd/smsd, e2e) are
// assembly roots and may import any internal package.
var allowedImports = map[string][]string{
	"httpapi":               {"tenant", "ratelimit", "template", "queue", "webhook", "store", "provider"},
	"queue":                 {"provider", "store"},
	"webhook":               {"provider", "store"},
	"tenant":                {"store"},
	"template":              {"store"},
	"ratelimit":             {},
	"store":                 {},
	"provider":              {},
	"provider/mock":         {"provider"},
	"provider/mitake":       {"provider"},
	"provider/providertest": {"provider"},
}

// testOnlyImports are edges allowed only from _test.go files: every adapter
// runs the shared contract (AD-1) in its tests, but adapter product code must
// not depend on the test helper.
var testOnlyImports = map[string][]string{
	"provider/mock":   {"provider/providertest"},
	"provider/mitake": {"provider/providertest"},
}

// repoRoot is the module root; go test runs with the package dir (e2e) as cwd.
const repoRoot = ".."

func goFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "_bmad", "_bmad-output", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			files = append(files, filepath.ToSlash(p))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// pkgOf returns the package key ("httpapi", "provider/mock") for a file under
// internal/, or "" for assembly roots (cmd, e2e).
func pkgOf(file string) string {
	rel := strings.TrimPrefix(file, repoRoot+"/")
	if !strings.HasPrefix(rel, "internal/") {
		return ""
	}
	return filepath.ToSlash(filepath.Dir(strings.TrimPrefix(rel, "internal/")))
}

func TestArchitecture_ImportGraph(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range goFiles(t) {
		pkg := pkgOf(f)
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, imp := range af.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(path, modulePath+"internal/") {
				continue
			}
			target := strings.TrimPrefix(path, modulePath+"internal/")
			if msg := importViolation(f, pkg, target); msg != "" {
				t.Error(msg)
			}
		}
	}
}

// importViolation returns why file (in internal/pkg) may not import
// internal/target, or "" when the edge is allowed.
func importViolation(file, pkg, target string) string {
	if pkg == "" {
		return "" // assembly roots may import anything internal
	}
	if pkg == target {
		return ""
	}
	allowed, known := allowedImports[pkg]
	if !known {
		return file + ": package internal/" + pkg + " is not in the architecture graph"
	}
	if strings.HasSuffix(file, "_test.go") {
		allowed = append(allowed[:len(allowed):len(allowed)], testOnlyImports[pkg]...)
	}
	for _, a := range allowed {
		if a == target {
			return ""
		}
	}
	return file + ": internal/" + pkg + " must not import internal/" + target
}

func TestArchitecture_TestOnlyImportsRejectedInProductCode(t *testing.T) {
	cases := []struct {
		file, target string
		allowed      bool
	}{
		{"../internal/provider/mock/mock_test.go", "provider/providertest", true},
		{"../internal/provider/mitake/mitake_test.go", "provider/providertest", true},
		{"../internal/provider/mock/mock.go", "provider/providertest", false},
		{"../internal/provider/mitake/mitake.go", "provider/providertest", false},
		{"../internal/queue/queue_test.go", "provider/providertest", false},
		{"../internal/provider/providertest/contract.go", "provider", true},
		{"../internal/provider/providertest/contract.go", "provider/mock", false},
	}
	for _, c := range cases {
		msg := importViolation(c.file, pkgOf(c.file), c.target)
		if got := msg == ""; got != c.allowed {
			t.Errorf("%s importing internal/%s: allowed=%v, want %v (%s)", c.file, c.target, got, c.allowed, msg)
		}
	}
}

func TestArchitecture_CmdOnlyImportsInternal(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range goFiles(t) {
		if !strings.HasPrefix(f, repoRoot+"/cmd/") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range af.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, modulePath) && !strings.HasPrefix(path, modulePath+"internal/") {
				t.Errorf("%s imports %s; cmd may only import internal/*", f, path)
			}
		}
	}
}

func TestArchitecture_AdaptersOnlyImportedByRoots(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range goFiles(t) {
		pkg := pkgOf(f)
		if pkg == "" || strings.HasPrefix(pkg, "provider/") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range af.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, modulePath+"internal/provider/") {
				t.Errorf("%s imports %s; only cmd/smsd and e2e/testutil may", f, path)
			}
		}
	}
}

// forbidden maps a source pattern to the files allowed to contain it. The
// patterns are built by concatenation so this file never matches itself.
var forbidden = map[string][]string{
	"http." + "Client{":       {},
	"http." + "DefaultClient": {},
	"time." + "Now(":          {"cmd/smsd/main.go"},
	"time." + "Sleep(":        {},
}

func TestArchitecture_ForbiddenCalls(t *testing.T) {
	self := "e2e/architecture_test.go"
	for _, f := range goFiles(t) {
		rel := strings.TrimPrefix(f, repoRoot+"/")
		if rel == self {
			continue
		}
		data, err := os.ReadFile(filepath.FromSlash(f))
		if err != nil {
			t.Fatal(err)
		}
		for pattern, allowed := range forbidden {
			if !strings.Contains(string(data), pattern) {
				continue
			}
			ok := false
			for _, a := range allowed {
				if a == rel {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s uses %q outside its allowed locations %v", rel, pattern, allowed)
			}
		}
	}
}

// godogModule is the only direct dependency go.mod may have (CH5 5-07, the
// optional godog Feature); everything it pulls in must stay "// indirect".
const godogModule = "github.com/cucumber/godog"

func TestArchitecture_GoModOnlyRequiresGodog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "require (":
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		case !inBlock:
			continue
		}
		if line == "" || strings.HasSuffix(line, "// indirect") {
			continue
		}
		if f := strings.Fields(line); f[0] != godogModule {
			t.Errorf("go.mod may only require %s directly, got %q", godogModule, line)
		}
	}
}

func TestArchitecture_GodogOnlyInFeatures(t *testing.T) {
	for _, f := range goFiles(t) {
		rel := strings.TrimPrefix(f, repoRoot+"/")
		if strings.HasPrefix(rel, "features/") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash(f), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			if strings.HasPrefix(strings.Trim(imp.Path.Value, `"`), godogModule) {
				t.Errorf("%s imports %s; only features/ may", rel, imp.Path.Value)
			}
		}
	}
}
