package analyzer

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			"simple import",
			"import os\nimport sys\n",
			[]string{"os", "sys"},
		},
		{
			"from import",
			"from collections import OrderedDict\nfrom pathlib import Path\n",
			[]string{"collections", "pathlib"},
		},
		{
			"dotted import",
			"import os.path\nimport xml.etree.ElementTree\n",
			[]string{"os.path", "xml.etree.ElementTree"},
		},
		{
			"multi import",
			"import os, sys, json\n",
			[]string{"json", "os", "sys"},
		},
		{
			"relative import single dot",
			"from . import utils\n",
			[]string{".utils"},
		},
		{
			"relative import double dot",
			"from ..helpers import clean\n",
			[]string{"..helpers"},
		},
		{
			"relative import dot only",
			"from . import config\n",
			[]string{".config"},
		},
		{
			"ignores comments",
			"# import fake\nimport real\n",
			[]string{"real"},
		},
		{
			"ignores inline comments",
			"import os  # operating system\n",
			[]string{"os"},
		},
		{
			"ignores strings",
			"x = 'import fake'\nimport real\n",
			[]string{"real"},
		},
		{
			"ignores indented imports",
			"if True:\n    import conditional\nimport toplevel\n",
			[]string{"toplevel"},
		},
		{
			"empty source",
			"",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseImports(tt.source)
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseImports() = %v, want %v", got, tt.want)
			}
		})
	}
}
