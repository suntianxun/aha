package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stephen/aha/model"
)

func Analyze(projectDir string) (*model.ProjectDeps, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a valid directory: %s", projectDir)
	}

	result := &model.ProjectDeps{
		ProjectPath: absDir,
		Modules:     make(map[string]*model.Module),
	}

	declaredDeps, warnings := LoadDeclaredDeps(absDir)
	result.Warnings = append(result.Warnings, warnings...)

	declaredMap := make(map[string]DeclaredDep)
	for _, d := range declaredDeps {
		declaredMap[d.Name] = d
	}

	pyFiles := make(map[string][]string)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "__pycache__" || base == ".tox" || base == "node_modules" || base == ".venv" || base == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}

		relPath, _ := filepath.Rel(absDir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		imports := ParseImports(string(data))
		pyFiles[relPath] = imports
		result.Modules[relPath] = &model.Module{RelPath: relPath}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	if len(pyFiles) == 0 {
		return nil, fmt.Errorf("no Python files found in %s", projectDir)
	}

	externalUsage := make(map[string]map[string]bool)

	for relPath, imports := range pyFiles {
		for _, imp := range imports {
			if strings.HasPrefix(imp, ".") {
				resolved := resolveRelativeImport(relPath, imp, result.Modules)
				if resolved != "" {
					result.Modules[relPath].Imports = append(result.Modules[relPath].Imports, resolved)
				}
			} else {
				topLevel := TopLevelModule(imp)
				if IsStdlib(topLevel) {
					continue
				}
				if internalPath := resolveAbsoluteImport(imp, result.Modules, absDir); internalPath != "" {
					result.Modules[relPath].Imports = append(result.Modules[relPath].Imports, internalPath)
				} else {
					normalizedName := strings.ToLower(strings.ReplaceAll(topLevel, "-", "_"))
					if externalUsage[normalizedName] == nil {
						externalUsage[normalizedName] = make(map[string]bool)
					}
					externalUsage[normalizedName][relPath] = true
				}
			}
		}
	}

	for relPath, mod := range result.Modules {
		for _, imp := range mod.Imports {
			if target, ok := result.Modules[imp]; ok {
				target.ImportedBy = append(target.ImportedBy, relPath)
			}
		}
	}

	allExtNames := make(map[string]bool)
	for name := range declaredMap {
		allExtNames[name] = true
	}
	for name := range externalUsage {
		allExtNames[name] = true
	}

	for name := range allExtNames {
		dep := model.ExternalDep{Name: name}
		if declared, ok := declaredMap[name]; ok {
			dep.VersionConstraint = declared.VersionConstraint
		}
		if files, ok := externalUsage[name]; ok {
			for f := range files {
				dep.UsedIn = append(dep.UsedIn, f)
			}
			sort.Strings(dep.UsedIn)
		}
		result.ExternalDeps = append(result.ExternalDeps, dep)
	}
	sort.Slice(result.ExternalDeps, func(i, j int) bool {
		return result.ExternalDeps[i].Name < result.ExternalDeps[j].Name
	})

	return result, nil
}

func resolveRelativeImport(fromFile string, imp string, modules map[string]*model.Module) string {
	dir := filepath.Dir(fromFile)

	dots := 0
	for _, c := range imp {
		if c == '.' {
			dots++
		} else {
			break
		}
	}
	name := imp[dots:]

	for i := 1; i < dots; i++ {
		dir = filepath.Dir(dir)
	}

	if name == "" {
		return ""
	}

	candidate := filepath.Join(dir, name+".py")
	if _, ok := modules[candidate]; ok {
		return candidate
	}

	candidate = filepath.Join(dir, name, "__init__.py")
	if _, ok := modules[candidate]; ok {
		return candidate
	}

	return ""
}

func resolveAbsoluteImport(imp string, modules map[string]*model.Module, projectDir string) string {
	parts := strings.Split(imp, ".")
	for i := len(parts); i > 0; i-- {
		candidate := filepath.Join(parts[:i]...) + ".py"
		if _, ok := modules[candidate]; ok {
			return candidate
		}
		initParts := make([]string, i+1)
		copy(initParts, parts[:i])
		initParts[i] = "__init__.py"
		candidate = filepath.Join(initParts...)
		if _, ok := modules[candidate]; ok {
			return candidate
		}
	}
	return ""
}
