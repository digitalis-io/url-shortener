package shorturl

import "testing"

func TestValidateAndNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "http", input: "http://example.com/a", wantErr: false},
		{name: "https", input: "https://example.com/a", wantErr: false},
		{name: "missing scheme", input: "example.com/a", wantErr: true},
		{name: "blocked scheme", input: "file:///tmp/a", wantErr: true},
		{name: "missing host", input: "https://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndNormalizeURL(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{name: "empty", alias: "", wantErr: false},
		{name: "valid", alias: "my-link_1", wantErr: false},
		{name: "too short", alias: "ab", wantErr: true},
		{name: "invalid char", alias: "bad.alias", wantErr: true},
		{name: "reserved", alias: "api", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAlias(tt.alias)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
