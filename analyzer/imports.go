package analyzer

import (
	"regexp"
	"strings"
)

var (
	reImport     = regexp.MustCompile(`^import\s+(.+)$`)
	reFromImport = regexp.MustCompile(`^from\s+(\.{0,3}\w*)\s+import\s+(.+)$`)
)

func ParseImports(source string) []string {
	var imports []string
	seen := make(map[string]bool)

	for _, line := range strings.Split(source, "\n") {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)

		if m := reFromImport.FindStringSubmatch(line); m != nil {
			fromPart := m[1]
			if fromPart == "" {
				continue
			}
			if strings.HasPrefix(fromPart, ".") {
				importNames := m[2]
				for _, name := range strings.Split(importNames, ",") {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					if idx := strings.Index(name, " as "); idx >= 0 {
						name = name[:idx]
					}
					name = strings.TrimSpace(name)
					key := fromPart + name
					if fromPart == "." || fromPart == ".." || fromPart == "..." {
						key = fromPart + name
					} else {
						key = fromPart
					}
					if !seen[key] {
						seen[key] = true
						imports = append(imports, key)
					}
				}
			} else {
				if !seen[fromPart] {
					seen[fromPart] = true
					imports = append(imports, fromPart)
				}
			}
		} else if m := reImport.FindStringSubmatch(line); m != nil {
			for _, name := range strings.Split(m[1], ",") {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				if idx := strings.Index(name, " as "); idx >= 0 {
					name = name[:idx]
				}
				name = strings.TrimSpace(name)
				if !seen[name] {
					seen[name] = true
					imports = append(imports, name)
				}
			}
		}
	}

	return imports
}

func TopLevelModule(name string) string {
	if idx := strings.Index(name, "."); idx >= 0 {
		return name[:idx]
	}
	return name
}
