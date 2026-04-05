package analyzer

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

type DeclaredDep struct {
	Name              string
	VersionConstraint string
}

type pyprojectData struct {
	Project struct {
		Dependencies []string `toml:"dependencies"`
	} `toml:"project"`
}

var reDepSplit = regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*(.*)$`)

func parseDependencyString(s string) DeclaredDep {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "["); idx >= 0 {
		end := strings.Index(s, "]")
		if end > idx {
			s = s[:idx] + s[end+1:]
		}
	}
	m := reDepSplit.FindStringSubmatch(s)
	if m == nil {
		return DeclaredDep{Name: s}
	}
	name := strings.ReplaceAll(m[1], "-", "_")
	return DeclaredDep{
		Name:              strings.ToLower(name),
		VersionConstraint: strings.TrimSpace(m[2]),
	}
}

func ParsePyprojectToml(path string) ([]DeclaredDep, error) {
	var data pyprojectData
	if _, err := toml.DecodeFile(path, &data); err != nil {
		return nil, err
	}
	var deps []DeclaredDep
	for _, d := range data.Project.Dependencies {
		dep := parseDependencyString(d)
		if dep.Name != "" {
			deps = append(deps, dep)
		}
	}
	return deps, nil
}

func ParseRequirementsTxt(path string) ([]DeclaredDep, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []DeclaredDep
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		dep := parseDependencyString(line)
		if dep.Name != "" {
			deps = append(deps, dep)
		}
	}
	return deps, scanner.Err()
}

func ParseSetupPy(path string) ([]DeclaredDep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	re := regexp.MustCompile(`install_requires\s*=\s*\[([\s\S]*?)\]`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return nil, nil
	}

	reQuoted := regexp.MustCompile(`["']([^"']+)["']`)
	matches := reQuoted.FindAllStringSubmatch(m[1], -1)

	var deps []DeclaredDep
	for _, match := range matches {
		dep := parseDependencyString(match[1])
		if dep.Name != "" {
			deps = append(deps, dep)
		}
	}
	return deps, nil
}

func LoadDeclaredDeps(dir string) ([]DeclaredDep, []string) {
	var warnings []string

	pyprojectPath := dir + "/pyproject.toml"
	if deps, err := ParsePyprojectToml(pyprojectPath); err == nil && len(deps) > 0 {
		return deps, warnings
	}

	setupPath := dir + "/setup.py"
	if deps, err := ParseSetupPy(setupPath); err == nil && len(deps) > 0 {
		return deps, warnings
	}

	reqPath := dir + "/requirements.txt"
	if deps, err := ParseRequirementsTxt(reqPath); err == nil && len(deps) > 0 {
		return deps, warnings
	}

	warnings = append(warnings, "No dependency config found (pyproject.toml, setup.py, or requirements.txt)")
	return nil, warnings
}
