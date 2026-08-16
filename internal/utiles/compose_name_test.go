package utiles

import "testing"

func TestValidateComposeName(t *testing.T) {
	tests := []struct {
		name    string
		project string
		wantErr bool
	}{
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"leading dot", ".hidden", true},
		{"path traversal", "../etc", true},
		{"absolute", "/etc", true},
		{"backslash", `..\etc`, true},
		{"slash", "a/b", true},
		{"too long", string(make([]byte, 129)), true},
		{"normal name", "nginx-proxy", false},
		{"with dots", "my.project", false},
		{"with underscore", "my_project-1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComposeName(tt.project)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateComposeName(%q) error = %v, wantErr = %v", tt.project, err, tt.wantErr)
			}
		})
	}
}
