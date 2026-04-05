package model

import (
	"encoding/json"
	"testing"
)

func TestProjectAnalysisRoundTrip(t *testing.T) {
	original := &ProjectAnalysis{
		ProjectPath: "/tmp/test",
		ProjectName: "mylib",
		Version:     "1.0.0",
		Description: "A test library",
		PythonRequires: ">=3.8",
		EntryPoints: []EntryPoint{{Name: "cli", Module: "mylib.cli", Function: "main"}},
		Files: []FileAnalysis{
			{
				RelPath: "mylib/core.py",
				LOC:     100,
				Classes: []ClassInfo{
					{
						Name:       "MyClass",
						Bases:      []string{"BaseClass"},
						Decorators: []string{"dataclass"},
						Methods: []FunctionInfo{
							{Name: "method1", Params: []string{"self", "x: int"}, ReturnType: "str", LOC: 5, Decorators: nil},
						},
						LOC:     30,
						LineNo:  10,
					},
				},
				Functions: []FunctionInfo{
					{Name: "helper", Params: []string{"x: int"}, ReturnType: "bool", LOC: 8, LineNo: 50, Decorators: []string{"cache"}},
				},
				Imports:    []string{"os", "typing"},
				AllExports: []string{"MyClass", "helper"},
				Constants:  []string{"VERSION"},
			},
		},
		ExternalDeps: []ExternalDep{
			{Name: "requests", VersionConstraint: ">=2.28", UsedIn: []string{"mylib/core.py"}},
		},
		Modules: map[string]*Module{
			"mylib/core.py": {RelPath: "mylib/core.py", Imports: []string{"mylib/utils.py"}, ImportedBy: nil},
		},
		Patterns: []PatternMatch{
			{Pattern: "Abstract Base Class", Location: "mylib/core.py:MyClass", Detail: "Inherits from ABC"},
		},
		Stats: ProjectStats{
			TotalFiles: 1, TotalLOC: 100, TotalClasses: 1, TotalFunctions: 2,
			TotalPackages: 1, TotalExternalDeps: 1,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ProjectAnalysis
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ProjectName != original.ProjectName {
		t.Errorf("got %q, want %q", decoded.ProjectName, original.ProjectName)
	}
	if len(decoded.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(decoded.Files))
	}
	if decoded.Files[0].Classes[0].Name != "MyClass" {
		t.Errorf("got class %q, want MyClass", decoded.Files[0].Classes[0].Name)
	}
	if decoded.Stats.TotalLOC != 100 {
		t.Errorf("got LOC %d, want 100", decoded.Stats.TotalLOC)
	}
}
