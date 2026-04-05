package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParsePyprojectToml(t *testing.T) {
	dir := t.TempDir()
	content := `
[project]
name = "mypackage"
version = "1.0.0"
dependencies = [
    "requests>=2.28.0",
    "pydantic~=2.0",
    "click",
]
`
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0644)

	deps, err := ParsePyprojectToml(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	want := []DeclaredDep{
		{Name: "click", VersionConstraint: ""},
		{Name: "pydantic", VersionConstraint: "~=2.0"},
		{Name: "requests", VersionConstraint: ">=2.28.0"},
	}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("got %+v, want %+v", deps, want)
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	content := `
requests>=2.28.0
flask==2.3.0
# this is a comment
numpy

-e ./local-package
`
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(content), 0644)

	deps, err := ParseRequirementsTxt(filepath.Join(dir, "requirements.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	want := []DeclaredDep{
		{Name: "flask", VersionConstraint: "==2.3.0"},
		{Name: "numpy", VersionConstraint: ""},
		{Name: "requests", VersionConstraint: ">=2.28.0"},
	}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("got %+v, want %+v", deps, want)
	}
}

func TestParseSetupPy(t *testing.T) {
	dir := t.TempDir()
	content := `
from setuptools import setup

setup(
    name="mypackage",
    install_requires=[
        "requests>=2.28.0",
        "click",
    ],
)
`
	os.WriteFile(filepath.Join(dir, "setup.py"), []byte(content), 0644)

	deps, err := ParseSetupPy(filepath.Join(dir, "setup.py"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	want := []DeclaredDep{
		{Name: "click", VersionConstraint: ""},
		{Name: "requests", VersionConstraint: ">=2.28.0"},
	}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("got %+v, want %+v", deps, want)
	}
}
