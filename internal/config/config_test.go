package config

import "testing"

func TestValidateSecretKey(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"empty", "", true},
		{"unresolved placeholder", "${secretKey}", true},
		{"too short", "12345678", true},
		{"all digits", "123456789012", true},
		{"weak default", "dockercopilot2024", true},
		{"weak common", "password123", true},
		{"strong mixed", "d0cker-Copilot-2026#", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretKey(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSecretKey(%q) error = %v, wantErr = %v", tt.secret, err, tt.wantErr)
			}
		})
	}
}
