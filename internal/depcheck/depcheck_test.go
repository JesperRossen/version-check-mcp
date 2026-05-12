// GREEN test (Wave 0). Enforces DEP-01 (locked direct-dep set) and DEP-02
// (no forbidden fixture libs). Fails the build the moment either invariant is
// violated, for the lifetime of the project.
// See .planning/REQUIREMENTS.md DEP-01, DEP-02.
package depcheck_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

var allowedDirectDeps = map[string]bool{
	"github.com/modelcontextprotocol/go-sdk": true,
	"github.com/hashicorp/golang-lru/v2":     true,
	"golang.org/x/sync":                      true,
	"golang.org/x/mod":                       true,
}

var forbiddenImportSubstrings = []string{
	"goldie",
	"cupaloy",
	"testify",
}

// repoRoot walks up from this test source file until a directory containing
// go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod walking up from test source")
		}
		dir = parent
	}
}

func TestDirectDepsLockedToFour(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		t.Fatalf("parse go.mod: %v", err)
	}

	var direct []string
	for _, r := range mf.Require {
		if r.Indirect {
			continue
		}
		direct = append(direct, r.Mod.Path)
	}
	sort.Strings(direct)

	if len(direct) != len(allowedDirectDeps) {
		t.Errorf("direct dep count = %d, want %d; got: %v", len(direct), len(allowedDirectDeps), direct)
	}
	for _, p := range direct {
		if !allowedDirectDeps[p] {
			t.Errorf("forbidden direct dependency: %q (DEP-01)", p)
		}
	}
	for want := range allowedDirectDeps {
		found := false
		for _, p := range direct {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required direct dependency missing: %q (DEP-01)", want)
		}
	}
}

func TestNoForbiddenFixtureLibs(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	scanDirs := []string{"cmd", "internal", "test"}
	for _, sub := range scanDirs {
		base := filepath.Join(root, sub)
		if _, err := os.Stat(base); err != nil {
			continue // optional directories
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Errorf("parse %s: %v", path, err)
				return nil
			}
			for _, imp := range f.Imports {
				val := strings.Trim(imp.Path.Value, `"`)
				for _, bad := range forbiddenImportSubstrings {
					if strings.Contains(val, bad) {
						t.Errorf("%s imports forbidden lib %q (DEP-02 forbids %q)", path, val, bad)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Errorf("walk %s: %v", base, err)
		}
	}
}
