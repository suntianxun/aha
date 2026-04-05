package analyzer

import "testing"

func TestIsStdlib(t *testing.T) {
	tests := []struct {
		name   string
		module string
		want   bool
	}{
		{"os is stdlib", "os", true},
		{"sys is stdlib", "sys", true},
		{"json is stdlib", "json", true},
		{"pathlib is stdlib", "pathlib", true},
		{"collections is stdlib", "collections", true},
		{"requests is not stdlib", "requests", false},
		{"flask is not stdlib", "flask", false},
		{"numpy is not stdlib", "numpy", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStdlib(tt.module); got != tt.want {
				t.Errorf("IsStdlib(%q) = %v, want %v", tt.module, got, tt.want)
			}
		})
	}
}
