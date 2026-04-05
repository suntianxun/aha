package analyzer

import (
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "testdata")
}

func TestAnalyze(t *testing.T) {
	dir := filepath.Join(testdataDir(), "sample_project")
	result, err := Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	sort.Slice(result.ExternalDeps, func(i, j int) bool {
		return result.ExternalDeps[i].Name < result.ExternalDeps[j].Name
	})

	if len(result.ExternalDeps) < 2 {
		t.Fatalf("expected at least 2 external deps, got %d", len(result.ExternalDeps))
	}

	foundRequests := false
	foundClick := false
	for _, dep := range result.ExternalDeps {
		switch dep.Name {
		case "requests":
			foundRequests = true
			if dep.VersionConstraint != ">=2.28.0" {
				t.Errorf("requests version = %q, want %q", dep.VersionConstraint, ">=2.28.0")
			}
			if len(dep.UsedIn) == 0 {
				t.Error("requests should have UsedIn entries")
			}
		case "click":
			foundClick = true
			if len(dep.UsedIn) == 0 {
				t.Error("click should have UsedIn entries")
			}
		}
	}
	if !foundRequests {
		t.Error("expected to find 'requests' in external deps")
	}
	if !foundClick {
		t.Error("expected to find 'click' in external deps")
	}

	if len(result.Modules) == 0 {
		t.Fatal("expected modules to be populated")
	}

	mainMod, ok := result.Modules["sample/main.py"]
	if !ok {
		t.Fatal("expected sample/main.py in modules")
	}
	hasUtils := false
	for _, imp := range mainMod.Imports {
		if imp == "sample/utils.py" {
			hasUtils = true
		}
	}
	if !hasUtils {
		t.Errorf("main.py should import utils.py, got imports: %v", mainMod.Imports)
	}

	utilsMod, ok := result.Modules["sample/utils.py"]
	if !ok {
		t.Fatal("expected sample/utils.py in modules")
	}
	hasMain := false
	for _, imp := range utilsMod.ImportedBy {
		if imp == "sample/main.py" {
			hasMain = true
		}
	}
	if !hasMain {
		t.Errorf("utils.py should be imported by main.py, got importedBy: %v", utilsMod.ImportedBy)
	}
}
