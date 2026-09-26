// internal/agent/personality_test.go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresetNamesOrderAndContent(t *testing.T) {
	want := []string{"standard", "silent"}
	got := PresetNames()
	if len(got) != len(want) {
		t.Fatalf("PresetNames() = %v", got)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("PresetNames()[%d] = %q, want %q", i, got[i], n)
		}
		text, _, err := LoadPersonality(n, "")
		if err != nil || strings.TrimSpace(text) == "" {
			t.Errorf("preset %q: text=%q err=%v", n, text, err)
		}
		if len(text) > 600 {
			t.Errorf("preset %q is %d bytes, cap is 600", n, len(text))
		}
	}
}

func TestLoadPersonalityDefaultsToStandard(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.md")
	text, truncated, err := LoadPersonality("", missing)
	if err != nil || truncated {
		t.Fatalf("err=%v truncated=%v", err, truncated)
	}
	standard, _, _ := LoadPersonality("standard", "")
	if text != standard {
		t.Errorf("default = %q, want standard preset", text)
	}
}

func TestLoadPersonalityDefaultFileWins(t *testing.T) {
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("custom persona"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, truncated, err := LoadPersonality("", f)
	if err != nil || truncated || text != "custom persona" {
		t.Errorf("got %q truncated=%v err=%v", text, truncated, err)
	}
}

func TestLoadPersonalityExplicitPathAndCap(t *testing.T) {
	f := filepath.Join(t.TempDir(), "big.md")
	if err := os.WriteFile(f, []byte(strings.Repeat("é", PersonalityCap+500)), 0o644); err != nil {
		t.Fatal(err)
	}
	text, truncated, err := LoadPersonality(f, "")
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Error("over-cap file must report truncated")
	}
	if n := len([]rune(text)); n != PersonalityCap {
		t.Errorf("rune count = %d, want %d", n, PersonalityCap)
	}
}

func TestLoadPersonalityErrors(t *testing.T) {
	if _, _, err := LoadPersonality("no-such-preset-or-file", ""); err == nil {
		t.Error("unknown name must error")
	}
	if _, _, err := LoadPersonality(filepath.Join(t.TempDir(), "missing.md"), ""); err == nil {
		t.Error("missing explicit path must error")
	}
}
