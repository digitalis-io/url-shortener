package shorturl

import "testing"

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(12)
	if err != nil {
		t.Fatalf("GenerateCode returned error: %v", err)
	}
	if len(code) != 12 {
		t.Fatalf("expected length 12, got %d", len(code))
	}
	for _, char := range code {
		if !containsRune(alphabet, char) {
			t.Fatalf("unexpected character %q in code %q", char, code)
		}
	}
}

func containsRune(value string, want rune) bool {
	for _, char := range value {
		if char == want {
			return true
		}
	}
	return false
}
