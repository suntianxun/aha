package analyzer

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/stephen/aha/model"
)

//go:embed analyze.py
var analyzeScript embed.FS

// pythonResult is the JSON structure returned by analyze.py.
type pythonResult struct {
	Type     string               `json:"type"`
	Phase    string               `json:"phase"`
	Current  int                  `json:"current"`
	Total    int                  `json:"total"`
	Detail   string               `json:"detail"`
	Files    []model.FileAnalysis `json:"files"`
	Patterns []model.PatternMatch `json:"patterns"`
}

// Analyze performs deep analysis of a Python project.
// Progress updates are sent to progressCh (can be nil).
func Analyze(projectDir string, progressCh chan<- model.AnalysisProgress) (*model.ProjectAnalysis, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a valid directory: %s", projectDir)
	}

	result := &model.ProjectAnalysis{
		ProjectPath: absDir,
		Modules:     make(map[string]*model.Module),
	}

	// Phase 1: Load declared deps from config files
	declaredDeps, warnings := LoadDeclaredDeps(absDir)
	result.Warnings = append(result.Warnings, warnings...)

	// Load project metadata from pyproject.toml
	loadProjectMeta(absDir, result)

	// Phase 2: Run Python AST analysis
	files, patterns, err := runPythonAnalysis(absDir, progressCh)
	if err != nil {
		return nil, fmt.Errorf("python analysis: %w", err)
	}
	result.Files = files
	result.Patterns = patterns

	// Phase 3: Build module map and resolve imports
	if progressCh != nil {
		progressCh <- model.AnalysisProgress{Phase: "resolve", Current: 0, Total: 1, Detail: "Resolving imports..."}
	}

	pyFiles := make(map[string][]string)
	for _, f := range files {
		result.Modules[f.RelPath] = &model.Module{RelPath: f.RelPath}
		pyFiles[f.RelPath] = f.Imports
	}

	declaredMap := make(map[string]DeclaredDep)
	for _, d := range declaredDeps {
		declaredMap[d.Name] = d
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

	// Build imported-by relationships
	for relPath, mod := range result.Modules {
		for _, imp := range mod.Imports {
			if target, ok := result.Modules[imp]; ok {
				target.ImportedBy = append(target.ImportedBy, relPath)
			}
		}
	}

	// Merge external deps
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
		if usageFiles, ok := externalUsage[name]; ok {
			for f := range usageFiles {
				dep.UsedIn = append(dep.UsedIn, f)
			}
			sort.Strings(dep.UsedIn)
		}
		result.ExternalDeps = append(result.ExternalDeps, dep)
	}
	sort.Slice(result.ExternalDeps, func(i, j int) bool {
		return result.ExternalDeps[i].Name < result.ExternalDeps[j].Name
	})

	// Compute stats
	result.Stats = computeStats(result)

	if progressCh != nil {
		progressCh <- model.AnalysisProgress{Phase: "resolve", Current: 1, Total: 1, Detail: "Done"}
	}

	return result, nil
}

func runPythonAnalysis(projectDir string, progressCh chan<- model.AnalysisProgress) ([]model.FileAnalysis, []model.PatternMatch, error) {
	scriptData, err := analyzeScript.ReadFile("analyze.py")
	if err != nil {
		return nil, nil, fmt.Errorf("reading embedded script: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "aha-analyze-*.py")
	if err != nil {
		return nil, nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(scriptData); err != nil {
		tmpFile.Close()
		return nil, nil, err
	}
	tmpFile.Close()

	cmd := exec.Command("python3", tmpFile.Name(), projectDir)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting python3: %w (is python3 installed?)", err)
	}

	var files []model.FileAnalysis
	var patterns []model.PatternMatch

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024) // 10MB buffer for large projects
	for scanner.Scan() {
		var msg pythonResult
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "progress":
			if progressCh != nil {
				pct := 0.0
				if msg.Total > 0 {
					pct = float64(msg.Current) / float64(msg.Total)
				}
				progressCh <- model.AnalysisProgress{
					Phase:   msg.Phase,
					Current: msg.Current,
					Total:   msg.Total,
					Detail:  msg.Detail,
					Percent: pct,
				}
			}
		case "result":
			files = msg.Files
			patterns = msg.Patterns
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, nil, fmt.Errorf("python analysis failed: %w", err)
	}

	return files, patterns, nil
}

type pyprojectMeta struct {
	Project struct {
		Name           string            `toml:"name"`
		Version        string            `toml:"version"`
		Description    string            `toml:"description"`
		RequiresPython string            `toml:"requires-python"`
		Scripts        map[string]string `toml:"scripts"`
	} `toml:"project"`
}

func loadProjectMeta(absDir string, result *model.ProjectAnalysis) {
	pyprojectPath := filepath.Join(absDir, "pyproject.toml")
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return
	}

	var meta pyprojectMeta
	if _, err := toml.Decode(string(data), &meta); err != nil {
		return
	}

	result.ProjectName = meta.Project.Name
	result.Version = meta.Project.Version
	result.Description = meta.Project.Description
	result.PythonRequires = meta.Project.RequiresPython

	for name, ref := range meta.Project.Scripts {
		parts := strings.SplitN(ref, ":", 2)
		ep := model.EntryPoint{Name: name}
		if len(parts) == 2 {
			ep.Module = parts[0]
			ep.Function = parts[1]
		} else {
			ep.Module = ref
		}
		result.EntryPoints = append(result.EntryPoints, ep)
	}

	// Check for __main__.py files
	filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "__pycache__" || base == ".venv" || base == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) == "__main__.py" {
			rel, _ := filepath.Rel(absDir, path)
			result.EntryPoints = append(result.EntryPoints, model.EntryPoint{
				Name:   rel,
				Module: rel,
			})
		}
		return nil
	})
}

func computeStats(result *model.ProjectAnalysis) model.ProjectStats {
	stats := model.ProjectStats{
		TotalFiles:        len(result.Files),
		TotalExternalDeps: len(result.ExternalDeps),
	}

	packages := make(map[string]bool)
	for _, f := range result.Files {
		stats.TotalLOC += f.LOC
		stats.TotalClasses += len(f.Classes)
		stats.TotalFunctions += len(f.Functions)
		for _, c := range f.Classes {
			stats.TotalFunctions += len(c.Methods)
		}
		pkg := filepath.Dir(f.RelPath)
		if pkg != "." {
			packages[pkg] = true
		}
	}
	stats.TotalPackages = len(packages)
	return stats
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
