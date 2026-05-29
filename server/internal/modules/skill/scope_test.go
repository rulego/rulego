package skill

import "testing"

// TestNormalizeSkillScopeDefaultsToGlobal ensures the API layer can omit scope
// for the first version while still reserving future scope extensions.
func TestNormalizeSkillScopeDefaultsToGlobal(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantScope string
		wantErr   bool
	}{
		{name: "empty defaults to global", input: "", wantScope: "global"},
		{name: "global stays global", input: "global", wantScope: "global"},
		{name: "private rejected", input: "private", wantErr: true},
		{name: "trim lower", input: "  GLOBAL ", wantScope: "global"},
		{name: "invalid rejected", input: "workspace", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSkillScope(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeSkillScope(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.wantScope {
				t.Fatalf("normalizeSkillScope(%q) = %q, want %q", tt.input, got, tt.wantScope)
			}
		})
	}
}

func TestGetConfiguredGlobalSkillPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "configured path wins", path: "./my-skills", want: "./my-skills"},
		{name: "blank falls back", path: "", want: "./skills"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getConfiguredGlobalSkillPath(tt.path)
			if got != tt.want {
				t.Fatalf("getConfiguredGlobalSkillPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
