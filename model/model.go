package model

// Module represents a Python file and its import relationships.
type Module struct {
	RelPath    string
	Imports    []string
	ImportedBy []string
}

// ExternalDep represents an external (third-party) dependency.
type ExternalDep struct {
	Name              string
	VersionConstraint string
	UsedIn            []string
}

// ProjectDeps holds the full analysis result for a Python project.
type ProjectDeps struct {
	ProjectPath  string
	ExternalDeps []ExternalDep
	Modules      map[string]*Module
	Warnings     []string
}
