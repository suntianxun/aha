package model

// ProjectAnalysis is the full analysis result for a Python project.
type ProjectAnalysis struct {
	ProjectPath    string             `json:"project_path"`
	ProjectName    string             `json:"project_name"`
	Version        string             `json:"version"`
	Description    string             `json:"description"`
	PythonRequires string             `json:"python_requires"`
	EntryPoints    []EntryPoint       `json:"entry_points"`
	Files          []FileAnalysis     `json:"files"`
	ExternalDeps   []ExternalDep      `json:"external_deps"`
	Modules        map[string]*Module `json:"modules"`
	Patterns       []PatternMatch     `json:"patterns"`
	Stats          ProjectStats       `json:"stats"`
	Warnings       []string           `json:"warnings"`
}

// EntryPoint represents a CLI entry point or __main__.py.
type EntryPoint struct {
	Name     string `json:"name"`
	Module   string `json:"module"`
	Function string `json:"function"`
}

// FileAnalysis holds per-file AST analysis results.
type FileAnalysis struct {
	RelPath    string         `json:"rel_path"`
	LOC        int            `json:"loc"`
	Classes    []ClassInfo    `json:"classes"`
	Functions  []FunctionInfo `json:"functions"`
	Imports    []string       `json:"imports"`
	AllExports []string       `json:"all_exports"`
	Constants  []string       `json:"constants"`
}

// ClassInfo describes a Python class.
type ClassInfo struct {
	Name       string         `json:"name"`
	Bases      []string       `json:"bases"`
	Decorators []string       `json:"decorators"`
	Methods    []FunctionInfo `json:"methods"`
	LOC        int            `json:"loc"`
	LineNo     int            `json:"line_no"`
}

// FunctionInfo describes a Python function or method.
type FunctionInfo struct {
	Name       string   `json:"name"`
	Params     []string `json:"params"`
	ReturnType string   `json:"return_type"`
	Decorators []string `json:"decorators"`
	LOC        int      `json:"loc"`
	LineNo     int      `json:"line_no"`
}

// ExternalDep represents an external (third-party) dependency.
type ExternalDep struct {
	Name              string   `json:"name"`
	VersionConstraint string   `json:"version_constraint"`
	UsedIn            []string `json:"used_in"`
}

// Module represents a Python file and its import relationships.
type Module struct {
	RelPath    string   `json:"rel_path"`
	Imports    []string `json:"imports"`
	ImportedBy []string `json:"imported_by"`
}

// PatternMatch describes a detected design pattern or code pattern.
type PatternMatch struct {
	Pattern  string `json:"pattern"`
	Location string `json:"location"`
	Detail   string `json:"detail"`
}

// ProjectStats holds aggregate statistics.
type ProjectStats struct {
	TotalFiles        int `json:"total_files"`
	TotalLOC          int `json:"total_loc"`
	TotalClasses      int `json:"total_classes"`
	TotalFunctions    int `json:"total_functions"`
	TotalPackages     int `json:"total_packages"`
	TotalExternalDeps int `json:"total_external_deps"`
}

// AnalysisProgress is emitted by the analyzer during analysis.
type AnalysisProgress struct {
	Phase   string  `json:"phase"`
	Current int     `json:"current"`
	Total   int     `json:"total"`
	Detail  string  `json:"detail"`
	Percent float64 `json:"percent"`
}
