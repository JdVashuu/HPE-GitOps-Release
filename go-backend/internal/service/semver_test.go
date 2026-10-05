package service

import (
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2.1.3", "2.1.3"},
		{"v2.1.3", "2.1.3"},
		{"V2.1.3", "2.1.3"},
		{"  v1.0.0  ", "1.0.0"},
		{"", ""},
	}

	for _, tt := range tests {
		got := NormalizeVersion(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeVersion(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNextPatchVersion(t *testing.T) {
	existing := map[string]struct{}{
		"2.1.1": {},
		"2.1.2": {},
		"2.1.3": {},
	}

	got := NextPatchVersion("2.1.3", existing)
	expected := "2.1.4"
	if got != expected {
		t.Errorf("NextPatchVersion() = %q; want %q", got, expected)
	}

	// Collision skip test
	existing["2.1.4"] = struct{}{}
	got2 := NextPatchVersion("2.1.3", existing)
	expected2 := "2.1.5"
	if got2 != expected2 {
		t.Errorf("NextPatchVersion() with collision = %q; want %q", got2, expected2)
	}
}
