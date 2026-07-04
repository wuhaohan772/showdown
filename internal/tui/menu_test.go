package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

func menuRoster() []agent.Adapter {
	return []agent.Adapter{
		{Key: "claude", DisplayName: "Claude Code", Models: []string{"haiku", "sonnet", "opus"}},
		{Key: "codex", DisplayName: "Codex"}, // no Models: flag-only adapter
	}
}

// personaFile that does not exist
func noPersonaFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "personality.md")
}

func TestMenuModelOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	want := []string{"(default)", "haiku", "sonnet", "opus"}
	if len(m.modelOpts[0]) != 4 {
		t.Fatalf("claude modelOpts = %v, want %v", m.modelOpts[0], want)
	}
	for i, w := range want {
		if m.modelOpts[0][i] != w {
			t.Errorf("modelOpts[0][%d] = %q, want %q", i, m.modelOpts[0][i], w)
		}
	}
	if len(m.modelOpts[1]) != 1 || m.modelOpts[1][0] != menuDefaultModel {
		t.Errorf("codex modelOpts = %v, want [(default)]", m.modelOpts[1])
	}
	if m.modelIdx != 0 || m.talkOn != true {
		t.Errorf("defaults: modelIdx=%d talkOn=%v, want 0 true", m.modelIdx, m.talkOn)
	}
}

func TestMenuModelPrefill(t *testing.T) {
	// flag value in the Models list → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", true, noPersonaFile(t))
	if m.modelOpts[0][m.modelIdx] != "sonnet" {
		t.Errorf("selected model = %q, want sonnet", m.modelOpts[0][m.modelIdx])
	}
	if m.talkOn {
		t.Error("presetQuiet=true should start talkOn=false")
	}
	// flag value not in any list → appended verbatim everywhere
	m = newMenuModel(menuRoster(), stats.Stats{}, "gpt-x", "", false, noPersonaFile(t))
	if got := m.modelOpts[0][m.modelInit[0]]; got != "gpt-x" {
		t.Errorf("claude prefill = %q, want gpt-x", got)
	}
	if got := m.modelOpts[1][m.modelInit[1]]; got != "gpt-x" {
		t.Errorf("codex prefill = %q, want gpt-x", got)
	}
}

func TestMenuPersonaOptions(t *testing.T) {
	// no user file: presets only, needler (index 0) selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	if m.personaOpts[0] != "needler" || m.personaIdx != 0 {
		t.Errorf("personaOpts[0]=%q idx=%d, want needler 0", m.personaOpts[0], m.personaIdx)
	}
	// user file exists: custom prepended and selected
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("be weird"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", false, f)
	if m.personaOpts[0] != menuCustomPersona || m.personaIdx != 0 {
		t.Errorf("with file: personaOpts[0]=%q idx=%d, want custom 0", m.personaOpts[0], m.personaIdx)
	}
	// --personality preset name → selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "degen", false, f)
	if m.personaOpts[m.personaIdx] != "degen" {
		t.Errorf("prefill persona = %q, want degen", m.personaOpts[m.personaIdx])
	}
	// --personality path → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "/tmp/evil.md", false, f)
	if m.personaOpts[m.personaIdx] != "/tmp/evil.md" {
		t.Errorf("prefill persona = %q, want /tmp/evil.md", m.personaOpts[m.personaIdx])
	}
}
